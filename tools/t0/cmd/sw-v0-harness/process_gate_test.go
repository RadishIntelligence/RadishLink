package main

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"radishlink.local/t0/internal/harness"
)

type gateFixturePeer struct {
	*actorPeer
	mode     string
	closeErr error
}

// Bypass the scheduling input only in the fixture. The real node still
// commits its Send decision, and the real adapter sees the gate closed.
func (p *gateFixturePeer) Exchange(ctx context.Context, q harness.ControlRequest) (harness.ControlResponse, error) {
	var step harness.NodeStep
	if q.Operation != "step" || json.Unmarshal(q.Detail, &step) != nil || len(step.Links) != 1 || step.Links[0].Event != "became_down" {
		return p.actorPeer.Exchange(ctx, q)
	}
	if p.mode == "missing-custody" {
		id := step.Inputs[0]
		input := p.actor.pending[id]
		delete(p.actor.pending, id)
		step.Inputs = []string{}
		q.Detail = rawDetail(step)
		r := p.actor.handle(ctx, q)
		frame := input.transport.Frame
		facts := r.Result.Facts
		observed := []harness.NodeFact{
			{Now: q.Now, Kind: "frame_received", Before: q.Generation, After: q.Generation, Transfer: id, Detail: rawDetail(frame)},
			{Now: q.Now, Kind: "verdict_decision", Before: q.Generation, After: q.Generation, Detail: rawDetail(harness.VerdictDetail{Receiver: "B", Neighbor: "A", Frame: frame.Frame, Purpose: "admission", Verdict: "accept"})},
		}
		r.Result.Facts = append(append(slices.Clone(facts[:1]), observed...), facts[1:]...)
		for i := range r.Result.Facts {
			f := &r.Result.Facts[i]
			if f.Kind == "batch_result" {
				var op harness.OperationDetail
				_ = json.Unmarshal(f.Detail, &op)
				op.Inputs = 1
				f.Detail = rawDetail(op)
			}
		}
		return r, nil
	}
	link := step.Links[0]
	p.actor.gates[link.Neighbor] = true
	step.Links = []harness.ControlLink{}
	q.Detail = rawDetail(step)
	r := p.actor.handle(ctx, q)
	gate := harness.NodeFact{Now: q.Now, Kind: "link_gate", Before: q.Generation, After: q.Generation, Detail: rawDetail(link)}
	r.Result.Facts = append([]harness.NodeFact{gate}, r.Result.Facts...)
	if q.Node == "A" {
		// At 1 ms quota and the physical gate both forbid the send. Record
		// the initial blocked slot; the next slot has quota and trips the guard.
		for i := range r.Result.Facts {
			f := &r.Result.Facts[i]
			if f.Kind == "retry_decision" {
				var retry harness.ObservedRetry
				_ = json.Unmarshal(f.Detail, &retry)
				if retry.Reason != "rejected_quota" || retry.Send {
					return r, errors.New("unexpected initial AB fixture slot")
				}
				retry.Reason = "blocked_link_down"
				f.Detail = rawDetail(retry)
			}
		}
	}
	return r, nil
}

func (p *gateFixturePeer) Close() error { return p.closeErr }

func TestGateFailureIsArchivedAndIndependentlyVerified(t *testing.T) {
	for _, tc := range []struct {
		name, link, node, want, assertion string
		cause                             error
		cleanup, wait                     bool
		code                              int
	}{
		{"BC missing custody", "BC", "B", "FAIL", "bc_gate_hit", errBCGateCustody, false, false, 1},
		{"BC send", "BC", "B", "FAIL", "closed_gate_send", errGateSend, false, false, 1},
		{"AB send", "AB", "A", "FAIL", "closed_gate_send", errGateSend, false, false, 1},
		{"BC send and cleanup failure", "BC", "B", "INVALID", "closed_gate_send", errGateSend, true, false, 2},
		{"AB send and wait failure", "AB", "A", "INVALID", "closed_gate_send", errGateSend, false, true, 2},
	} {
		t.Run(tc.name, func(t *testing.T) {
			s, r, m := memorySample(t, "SW-V1-DOWN-"+tc.link+"-001", "p1024", 1)
			m.Observation = harness.NetworkObservationVersion
			peer := &gateFixturePeer{actorPeer: s.peers[tc.node].(*actorPeer)}
			if tc.cause == errBCGateCustody {
				peer.mode = "missing-custody"
			}
			if tc.wait {
				peer.closeErr = errors.New("fixture wait failure")
			}
			s.peers[tc.node] = peer
			callErr := s.drive()
			if !errors.Is(callErr, tc.cause) || s.now >= 30000 {
				t.Fatalf("stop at %d: %v", s.now, callErr)
			}
			if len(peer.actor.handles) != 0 || len(peer.actor.sends) != 0 {
				t.Fatal("aborted batch published send handles")
			}
			r.failRemove = tc.cleanup
			root := t.TempDir()
			dest := filepath.Join(root, "bundle")
			budget := outputBudget(t, 32<<20, func() error { return nil })
			saved, err := s.finish(m, dest, root, budget, budget, callErr)
			var exit *networkExit
			if !saved || !errors.As(err, &exit) || exit.code != tc.code || !errors.Is(err, tc.cause) {
				t.Fatalf("saved=%v exit=%v error=%v", saved, exit, err)
			}
			if err := harness.VerifyEvidence(dest); err != nil {
				t.Fatal("independent verification", err)
			}
			var manifest harness.NetworkManifest
			readFixtureJSON(t, filepath.Join(dest, "manifest.json"), &manifest)
			var assertions harness.ScenarioAssertions
			readFixtureJSON(t, filepath.Join(dest, "assertions.json"), &assertions)
			if manifest.Result != tc.want || manifest.Exit != int64(tc.code) || manifest.Observation != harness.NetworkObservationVersion || !slices.ContainsFunc(assertions.Items, func(a harness.ScenarioAssertion) bool {
				return a.ID == tc.assertion && !a.Passed && a.Category == "implementation"
			}) {
				t.Fatalf("result %s/%d assertions %+v", manifest.Result, manifest.Exit, assertions.Items)
			}
			if slices.ContainsFunc(s.events, func(e harness.NetworkEvent) bool {
				return e.Kind == "observation_end" || (e.Kind == "frame_written" && e.Node == tc.node)
			}) {
				t.Fatal("abort fabricated completion or wrote from the blocked node")
			}
		})
	}
}

func readFixtureJSON(t *testing.T, path string, dst any) {
	t.Helper()
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(raw, dst); err != nil {
		t.Fatal(err)
	}
}

func TestBCGateWrongDurationIsInvalid(t *testing.T) {
	s, _, m := memorySample(t, "SW-V1-DOWN-BC-001", "p1024", 1)
	m.Observation = harness.NetworkObservationVersion
	canonical := s.profile
	// Inject a coherent 5050 ms gate interval in the supervisor fixture.
	// The node and archived profile retain the canonical 5000 ms contract.
	s.profile.Fault.Duration += 50
	callErr := s.drive()
	s.profile = canonical
	if callErr != nil {
		t.Fatal(callErr)
	}
	res, err := s.cleanup()
	if err != nil {
		t.Fatal(err)
	}
	_, assertions, result, err := harness.AssessNetwork(s.bundle(m, res))
	if err != nil || result != "INVALID" || slices.ContainsFunc(assertions.Items, func(a harness.ScenarioAssertion) bool { return a.Category == "implementation" && !a.Passed }) {
		t.Fatalf("wrong duration should only fail the environment: %s %v %+v", result, err, assertions.Items)
	}
}

func TestBCGateObservationVersionsCompareSeparately(t *testing.T) {
	root := t.TempDir()
	mixed := t.TempDir()
	for repeat := 1; repeat <= 3; repeat++ {
		b := memoryBundle(t, "SW-V1-DOWN-BC-001", "p1024", repeat)
		b.Manifest.Observation = harness.NetworkObservationVersion
		dir := fmt.Sprint(repeat)
		if err := harness.WriteNetworkBundle(filepath.Join(root, dir), b); err != nil {
			t.Fatal(err)
		}
		if repeat == 3 {
			b.Manifest.Observation = 1
		}
		if err := harness.WriteNetworkBundle(filepath.Join(mixed, dir), b); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := harness.CompareEvidenceRuns(root); err != nil {
		t.Fatal("V3 observation 2 compare", err)
	}
	if _, err := harness.CompareEvidenceRuns(mixed); err == nil {
		t.Fatal("mixed observation versions compared equal")
	}
}

func TestBCGateRejectsRehashedWrongBatch(t *testing.T) {
	b := memoryBundle(t, "SW-V1-DOWN-BC-001", "p1024", 1)
	b.Manifest.Observation = harness.NetworkObservationVersion
	root := filepath.Join(t.TempDir(), "bundle")
	if err := harness.WriteNetworkBundle(root, b); err != nil {
		t.Fatal(err)
	}
	if err := harness.VerifyEvidence(root); err != nil {
		t.Fatal(err)
	}
	var down, later int64
	for _, e := range b.Events {
		if e.Kind == "fault_transition" && down == 0 {
			down = e.Sources[0]
		}
		if down != 0 && e.Node == "B" && e.Kind == "state_observed" && e.Sources[0] > down {
			later = e.Sources[0]
			break
		}
	}
	if down == 0 || later == 0 {
		t.Fatal("missing test exchange")
	}
	// Move the entire raw gate request and fact to a later B exchange.
	// Older verification accepted the early semantic fault event because
	// the cited raw exchange contained a gate of the right direction.
	var original, replacement harness.ControlExchange
	if err := json.Unmarshal(b.Execution[down-1].Detail, &original); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b.Execution[later-1].Detail, &replacement); err != nil {
		t.Fatal(err)
	}
	var early, late harness.NodeStep
	if err := json.Unmarshal(original.Request.Detail, &early); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(replacement.Request.Detail, &late); err != nil {
		t.Fatal(err)
	}
	late.Links, early.Links = early.Links, late.Links
	original.Request.Detail, replacement.Request.Detail = rawDetail(early), rawDetail(late)
	gate := original.Response.Result.Facts[0]
	gate.Now, gate.Before, gate.After = replacement.Request.Now, replacement.Request.Generation, replacement.Request.Generation
	original.Response.Result.Facts = original.Response.Result.Facts[1:]
	replacement.Response.Result.Facts = append([]harness.NodeFact{gate}, replacement.Response.Result.Facts...)
	b.Execution[down-1].Detail, b.Execution[later-1].Detail = rawDetail(original), rawDetail(replacement)
	for i := range b.Events {
		e := &b.Events[i]
		if e.Kind == "fault_transition" && slices.Equal(e.Sources, []int64{down}) {
			e.Sources = []int64{later}
		}
	}
	legacy := b
	legacy.Manifest.Observation = 1
	if _, _, result, err := harness.AssessNetwork(legacy); err != nil || result != "PASS" {
		t.Fatalf("fixture does not expose the legacy source gap: %s %v", result, err)
	}
	var events, execution bytes.Buffer
	for _, e := range b.Events {
		events.Write(rawDetail(e))
		events.WriteByte('\n')
	}
	for _, x := range b.Execution {
		execution.Write(rawDetail(x))
		execution.WriteByte('\n')
	}
	var manifest harness.NetworkManifest
	readFixtureJSON(t, filepath.Join(root, "manifest.json"), &manifest)
	manifest.EventsHash = scenarioHash(events.Bytes())
	manifest.ExecutionHash = scenarioHash(execution.Bytes())
	for name, data := range map[string][]byte{"events.ndjson": events.Bytes(), "execution.ndjson": execution.Bytes(), "manifest.json": append(rawDetail(manifest), '\n')} {
		if err := os.WriteFile(filepath.Join(root, name), data, 0600); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Fatal(err)
	}
	var checksums strings.Builder
	for _, entry := range entries {
		if entry.IsDir() || entry.Name() == "checksums.sha256" {
			continue
		}
		raw, err := os.ReadFile(filepath.Join(root, entry.Name()))
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(&checksums, "%s  %s\n", scenarioHash(raw), entry.Name())
	}
	if err := os.WriteFile(filepath.Join(root, "checksums.sha256"), []byte(checksums.String()), 0600); err != nil {
		t.Fatal(err)
	}
	if err := harness.VerifyEvidence(root); err == nil || !strings.Contains(err.Error(), "gate transition source or time mismatch") {
		t.Fatalf("wrong-batch reference was not rejected by causal verification: %v", err)
	}
}
