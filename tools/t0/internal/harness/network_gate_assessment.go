package harness

import (
	"errors"
	"slices"
)

type networkGateReview struct {
	down, up, trigger int64 // execution references
	send, sendEvent   int64 // execution reference and semantic sequence
	bcHit             bool
	bcMissing         bool
	injectionOK       bool
}

// reviewNetworkGate uses only the validated execution ledger. The semantic
// events must project the same gate transitions before they can drive retries.
func reviewNetworkGate(b NetworkBundle, commands map[int64]ControlExchange, ingress map[string]TransportFact, missingCustodyAbort bool) (networkGateReview, error) {
	g := networkGateReview{injectionOK: true}
	if b.Profile.Fault.Kind != "down" {
		return g, nil
	}
	bc := b.Profile.Fault.Direction == "b-to-c"
	sender, neighbor := "A", "B"
	if bc {
		sender, neighbor = "B", "C"
	}
	for _, x := range b.Execution {
		ex, ok := commands[x.Sequence]
		q, r := ex.Request, ex.Response
		if !ok || q.Operation != "step" || q.Node != sender {
			continue
		}
		step, err := networkDetail[NodeStep](q.Detail)
		if err != nil {
			return g, err
		}
		if g.trigger == 0 && ((!bc && q.Now == 1) || (bc && slices.ContainsFunc(step.Inputs, func(id string) bool {
			f := ingress[id]
			return f.Sender == "A" && f.Receiver == "B" && f.Frame.Direction == "a-to-b" && f.Frame.Kind == "data"
		}))) {
			g.trigger = x.Sequence
		}
		// Link facts must be a prefix of the response, before receipt and
		// retry decisions, with an exact one-to-one request binding.
		links := []ControlLink{}
		var committed, snapshot, firstBlocked, hasCustody bool
		var sendCount int
		for i, f := range r.Result.Facts {
			switch f.Kind {
			case "link_gate":
				link, err := networkDetail[ControlLink](f.Detail)
				if err != nil || link.Neighbor != neighbor || i != len(links) || f.Before != q.Generation || f.After != q.Generation {
					return g, errors.New("gate request/fact order or generation")
				}
				links = append(links, link)
				switch {
				case link.Event == "became_down" && g.down == 0:
					g.down = x.Sequence
				case link.Event == "became_up" && g.down != 0 && g.up == 0 && g.down != x.Sequence:
					g.up = x.Sequence
				default:
					return g, errors.New("gate transition sequence")
				}
			case "batch_result":
				op, err := networkDetail[OperationDetail](f.Detail)
				if err != nil {
					return g, err
				}
				committed = op.Committed && op.Error == "" && f.Before == q.Generation && f.After == q.Generation+1 && f.After == r.Generation && (!bc || g.trigger != x.Sequence || slices.Contains(op.Transactions, "T-B"))
			case "state_observed":
				state, err := networkDetail[ObservedState](f.Detail)
				if err != nil {
					return g, err
				}
				snapshot = state.Generation == q.Generation+1 && state.Generation == r.Generation
				if bc && g.trigger == x.Sequence {
					m, queue := messageAt(state), queueAt(state, "data")
					hasCustody = m != nil && m.Key == keyFixture() && m.State == "custody_held"
					snapshot = snapshot && hasCustody && m.Transport && m.Deadline == 30000 && queue != nil && queue.Key == m.Key && queue.Neighbor == neighbor && queue.Start == q.Now && queue.Timed == 1 && queue.Recovery == 0
				}
			case "retry_decision":
				d, err := networkDetail[ObservedRetry](f.Detail)
				if err != nil {
					return g, err
				}
				if d.Kind != "data" || d.Neighbor != neighbor || d.Key != keyFixture() || f.Before != q.Generation || f.After != q.Generation+1 {
					continue
				}
				firstBlocked = d.Attempt && !d.Send && d.Reason == "blocked_link_down" && d.Start == q.Now && d.TimedBefore == 0 && d.TimedAfter == 1 && d.RecoveryBefore == 0 && d.RecoveryAfter == 0
				if d.Attempt && d.Send && d.Reason == "send" {
					sendCount++
				}
			}
		}
		if !slices.Equal(step.Links, links) {
			return g, errors.New("gate request/fact mismatch")
		}
		if bc && g.trigger == x.Sequence && g.down == x.Sequence {
			g.bcHit = q.Now <= 1000 && committed && snapshot && firstBlocked
			g.bcMissing = r.Error == "" && !hasCustody && !committed && len(r.Result.Sends) == 0
		}
		if r.Error == SendWhileGateDown {
			if g.send != 0 || g.down == 0 || g.up != 0 || !committed || !snapshot || sendCount != 1 || len(r.Result.Sends) != 0 {
				return g, errors.New("closed gate send lacks committed retry evidence")
			}
			g.send = x.Sequence
		}
	}
	if (!bc && g.down == 0) || g.down != g.trigger || (g.down != 0 && g.send == 0 && g.up == 0 && !(missingCustodyAbort && g.bcMissing)) || (g.up != 0 && b.Execution[g.up-1].Now != b.Execution[g.down-1].Now+b.Profile.Fault.Duration) {
		g.injectionOK = false
	}
	var semanticDown, semanticUp int64
	for _, e := range b.Events {
		switch e.Kind {
		case "fault_transition":
			f, err := networkDetail[FaultDetail](e.Detail)
			if err != nil {
				return g, err
			}
			switch {
			case f.Action == "down" && semanticDown == 0 && g.down > 0 && e.Now == b.Execution[g.down-1].Now && slices.Equal(e.Sources, []int64{g.down}):
				semanticDown = e.Sequence
			case f.Action == "up" && semanticUp == 0 && g.up > 0 && e.Now == b.Execution[g.up-1].Now && slices.Equal(e.Sources, []int64{g.up}):
				semanticUp = e.Sequence
			default:
				return g, errors.New("gate transition source or time mismatch")
			}
		case "retry_decision":
			if g.send == 0 || e.Node != sender || !slices.Equal(e.Sources, []int64{g.send}) {
				continue
			}
			d, err := networkDetail[ObservedRetry](e.Detail)
			if err != nil {
				return g, err
			}
			if d.Kind == "data" && d.Neighbor == neighbor && d.Send {
				if semanticDown == 0 || semanticUp != 0 || g.sendEvent != 0 {
					return g, errors.New("closed gate send event order")
				}
				g.sendEvent = e.Sequence
			}
		}
	}
	if (g.down != 0) != (semanticDown != 0) || (g.up != 0) != (semanticUp != 0) || (g.send != 0) != (g.sendEvent != 0) {
		return g, errors.New("missing gate transition or send event")
	}
	return g, nil
}
