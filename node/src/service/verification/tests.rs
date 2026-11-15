use super::*;
use crate::{
    config::Limits,
    retry::Link,
    store::{
        ControlAdmission, ControlIngressAdmission, ControlIngressChannel,
        ControlVerificationOutcome,
        tests::{
            ACTIVITY, Workspace, clock, control_object, fail_next_control_commit,
            pending_control_acks, test_credential,
        },
    },
};
use std::error::Error as _;

const WAIT: Duration = Duration::from_secs(10);
const SLOTS: usize = 16;

type Shutdown = Box<dyn FnMut(&mut Store, OwnerShutdownWork) -> Result<()> + Send>;

struct SyntheticDriver {
    tasks: Sender<ControlVerificationDispatch>,
    reports: Sender<Result<Completion>>,
    shutdown: Shutdown,
}

impl VerificationDriver for SyntheticDriver {
    fn neighbors(&self) -> &[ControlNeighbor] {
        &[ControlNeighbor {
            participant: [2; 16],
            link: Link::Up,
        }]
    }

    fn dispatch(&mut self, task: ControlVerificationDispatch) {
        self.tasks.send(task).unwrap();
    }

    fn report(&mut self, result: Result<Completion>) {
        self.reports.send(result).unwrap();
    }

    fn stop_and_join(&mut self, store: &mut Store, work: OwnerShutdownWork) -> Result<()> {
        (self.shutdown)(store, work)
    }
}

fn start(
    store: Store,
    shutdown: impl FnMut(&mut Store, OwnerShutdownWork) -> Result<()> + Send + 'static,
) -> (
    Service,
    mpsc::Receiver<ControlVerificationDispatch>,
    mpsc::Receiver<Result<Completion>>,
) {
    let (tasks, dispatched) = mpsc::channel();
    let (reports, completed) = mpsc::channel();
    let driver = SyntheticDriver {
        tasks,
        reports,
        shutdown: Box::new(shutdown),
    };
    let service = Service::start_inner(store, None, Some(Box::new(driver))).unwrap();
    (service, dispatched, completed)
}

fn receive<T>(rx: &mpsc::Receiver<T>) -> T {
    rx.recv_timeout(WAIT).unwrap()
}

fn with_store<T: Send + 'static>(
    service: &Service,
    read: impl FnOnce(&mut Store) -> T + Send + 'static,
) -> T {
    let (tx, rx) = mpsc::channel();
    service
        .enqueue(Command::WithStore(Box::new(move |store| {
            tx.send(read(store)).unwrap();
        })))
        .unwrap();
    receive(&rx)
}

fn register(store: &mut Store) -> ControlIngressChannel {
    store
        .register_control_ingress_channel(ACTIVITY, test_credential(1), test_credential(2))
        .unwrap()
}

fn verified(id: u8) -> ControlVerificationOutcome {
    ControlVerificationOutcome::Verified(Box::new(control_object(4, 5, id)))
}

#[test]
fn owner_loop_full_mailbox_and_leases_can_stop_and_collect_late_returns() {
    let workspace = Workspace::new();
    let mut store = workspace.create();
    for round in 1..=4 {
        store.advance_clock(clock(round * 1000)).unwrap();
        for peer in 2..=5 {
            assert_eq!(
                store
                    .queue_control_candidate(ACTIVITY, test_credential(peer), &[round as u8])
                    .unwrap(),
                ControlIngressAdmission::Queued
            );
        }
    }
    store.advance_clock(clock(5000)).unwrap();
    let (send_worker, worker) = mpsc::channel::<JoinHandle<()>>();
    let (stop_requested, worker_stopping) = mpsc::channel();
    let (service, tasks, reports) = start(store, move |store, work| {
        assert!(work.channels.is_empty());
        assert_eq!(store.control_ingress_usage().verifying, SLOTS);
        assert_eq!(store.control_ingress_usage().waiting, 0);
        assert_eq!(
            store.preflight_activity_switch().unwrap_err().code,
            Code::Stopped
        );
        stop_requested.send(()).unwrap();
        receive(&worker).join().unwrap();
        Ok(())
    });
    let leased: Vec<_> = (0..SLOTS).map(|_| receive(&tasks)).collect();
    assert_eq!(
        with_store(&service, |store| store
            .queue_control_candidate(ACTIVITY, test_credential(2), &[99])
            .unwrap()),
        ControlIngressAdmission::QueueFull
    );
    send_worker
        .send(thread::spawn(move || {
            receive(&worker_stopping);
            for task in leased {
                // A completed prepared result during stop must release the lease
                // without using the mailbox or creating a durable object.
                let outcome = verified(task.job().cose()[0]);
                task.return_result(outcome).unwrap();
            }
        }))
        .unwrap();
    let (entered, holding) = mpsc::channel();
    let (release, released) = mpsc::channel();
    service.enqueue(Command::Hold(entered, released)).unwrap();
    receive(&holding);
    let mut replies = Vec::new();
    for _ in 0..COMMAND_CAPACITY {
        let (tx, rx) = mpsc::channel();
        service.enqueue(Command::Diagnostics(tx)).unwrap();
        replies.push(rx);
    }
    let client = service.client();
    assert_eq!(client.status().unwrap_err().code, Code::Busy);
    assert!(!client.gate.close());
    assert_eq!(client.status().unwrap_err().code, Code::Stopped);
    release.send(()).unwrap();
    for _ in 0..SLOTS {
        assert_eq!(receive(&reports).unwrap_err().code, Code::Stopped);
    }
    for reply in replies {
        receive(&reply).unwrap();
    }
    service.stop().unwrap();
    let reopened = workspace.reopen(5000);
    assert_eq!(reopened.control_usage().unwrap().objects, 0);
    assert_eq!(pending_control_acks(&reopened), 0);
    reopened.close().unwrap();
}

#[test]
fn owner_loop_return_wakes_without_a_command_and_only_current_instance_commits() {
    let workspace = Workspace::new();
    let mut store = workspace.create();
    store.advance_clock(clock(2000)).unwrap();
    let old = register(&mut store);
    store
        .queue_control_candidate_on_channel(&old, &[7])
        .unwrap();
    assert_eq!(
        store
            .queue_control_candidate_on_channel(&old, &[7])
            .unwrap(),
        (
            ControlIngressAdmission::AlreadyPending,
            OutboundAckAdmission::NotEligible
        )
    );
    let (replacement_tx, replacement_rx) = mpsc::channel::<ControlIngressChannel>();
    let (service, tasks, reports) = start(store, move |store, work| {
        let replacement = receive(&replacement_rx);
        assert_eq!(work.channels, vec![replacement.binding()]);
        store.close_control_ingress_channel(&replacement)
    });
    let revision = with_store(&service, |store| store.revision().unwrap());
    let old_task = receive(&tasks);
    let old_binding = old.binding();
    assert_eq!(old_task.job().channel(), Some(old_binding));
    let replacement_binding = with_store(&service, move |store| {
        store.begin_control_ingress_channel_close(&old).unwrap();
        store.close_control_ingress_channel(&old).unwrap();
        store.advance_clock(clock(3000)).unwrap();
        let replacement = register(store);
        assert_eq!(
            store
                .queue_control_candidate_on_channel(&replacement, &[7])
                .unwrap()
                .0,
            ControlIngressAdmission::Queued
        );
        assert_eq!(
            store
                .queue_control_candidate(ACTIVITY, test_credential(3), &[7])
                .unwrap(),
            ControlIngressAdmission::Queued
        );
        let binding = replacement.binding();
        replacement_tx.send(replacement).unwrap();
        binding
    });
    let new_task = receive(&tasks);
    let duplicate_task = receive(&tasks);
    assert_eq!(new_task.job().channel(), Some(replacement_binding));
    assert_eq!(duplicate_task.job().neighbor(), test_credential(3));
    old_task.return_result(verified(7)).unwrap();
    assert_eq!(receive(&reports).unwrap_err().code, Code::Conflict);
    assert_eq!(
        with_store(&service, |store| (
            store.control_ingress_usage().verifying,
            store.revision().unwrap(),
            pending_control_acks(store),
        )),
        (2, revision + 1, 0)
    );
    new_task.return_result(verified(7)).unwrap();
    assert_eq!(
        receive(&reports).unwrap(),
        (
            ControlVerificationCompletion::Accepted(ControlAdmission::Inserted([7; 32])),
            OutboundAckAdmission::Queued
        )
    );
    assert_eq!(
        with_store(&service, |store| (
            store.control_ingress_usage().verifying,
            store.revision().unwrap(),
            pending_control_acks(store),
        )),
        (1, revision + 2, 1)
    );
    duplicate_task.return_result(verified(7)).unwrap();
    assert_eq!(
        receive(&reports).unwrap(),
        (
            ControlVerificationCompletion::Accepted(ControlAdmission::Merged([7; 32])),
            OutboundAckAdmission::NotEligible
        )
    );
    assert_eq!(
        with_store(&service, |store| (
            store.control_ingress_usage().verifying,
            store.control_usage().unwrap().objects,
            pending_control_acks(store),
        )),
        (0, 1, 1)
    );
    assert!(matches!(tasks.try_recv(), Err(TryRecvError::Empty)));
    assert!(matches!(reports.try_recv(), Err(TryRecvError::Empty)));
    service.stop().unwrap();
}

#[test]
fn owner_loop_commit_errors_preserve_cause_and_fence_subsequent_results() {
    for (committed, code, operation) in [
        (false, Code::StoreUnavailable, "test before commit"),
        (true, Code::OutcomeUnknown, "test lost commit response"),
    ] {
        let workspace = Workspace::new();
        let mut store = workspace.create();
        store.advance_clock(clock(2000)).unwrap();
        let channel = register(&mut store);
        for id in [7, 8] {
            assert_eq!(
                store
                    .queue_control_candidate_on_channel(&channel, &[id])
                    .unwrap()
                    .0,
                ControlIngressAdmission::Queued
            );
        }
        let (service, tasks, reports) = start(store, move |store, work| {
            assert_eq!(work.channels, vec![channel.binding()]);
            store.close_control_ingress_channel(&channel)
        });
        let first = receive(&tasks);
        let second = receive(&tasks);
        assert_eq!(first.job().cose(), &[7]);
        assert_eq!(second.job().cose(), &[8]);
        let revision = with_store(&service, move |store| {
            fail_next_control_commit(store, committed);
            store.revision().unwrap()
        });
        first.return_result(verified(7)).unwrap();
        let error = receive(&reports).unwrap_err();
        assert_eq!((error.code, error.operation), (code, operation));
        second.return_result(verified(8)).unwrap();
        assert_eq!(receive(&reports).unwrap_err().code, Code::OutcomeUnknown);
        assert_eq!(service.status().unwrap(), Status::Quarantined);
        assert_eq!(
            with_store(&service, |store| (
                store.control_ingress_usage().verifying,
                pending_control_acks(store)
            )),
            (0, 0)
        );
        service.stop().unwrap();
        let reopened = workspace.reopen(2000);
        assert_eq!(
            reopened.control_usage().unwrap().objects,
            u64::from(committed)
        );
        assert_eq!(
            reopened.revision().unwrap(),
            revision + i64::from(committed) + 1
        );
        assert_eq!(pending_control_acks(&reopened), 0);
    }
}

#[test]
fn owner_loop_join_failure_retains_unreturned_leases_and_fences_activity() {
    let workspace = Workspace::new();
    let mut store = workspace.create();
    store.advance_clock(clock(1000)).unwrap();
    for peer in 2..=4 {
        store
            .queue_control_candidate(ACTIVITY, test_credential(peer), &[peer])
            .unwrap();
    }
    let (stopping, at_shutdown) = mpsc::channel();
    let (fail_join, finish_shutdown) = mpsc::channel();
    let (service, tasks, reports) = start(store, move |store, _| {
        assert_eq!(store.control_ingress_usage().verifying, 3);
        stopping.send(()).unwrap();
        receive(&finish_shutdown);
        Err(Error::source_error(
            Code::Stopped,
            "synthetic verifier Join failed",
            std::io::Error::from_raw_os_error(5),
        ))
    });
    drop(receive(&tasks));
    assert_eq!(
        receive(&reports).unwrap_err().operation,
        "control verification result sender disconnected"
    );
    let late = receive(&tasks);
    let still_connected = receive(&tasks);
    let (entered, holding) = mpsc::channel();
    let (release, released) = mpsc::channel();
    service.enqueue(Command::Hold(entered, released)).unwrap();
    receive(&holding);
    let (snapshot, checked) = mpsc::channel();
    service
        .enqueue(Command::WithStore(Box::new(move |store| {
            snapshot
                .send((
                    store.control_ingress_usage().verifying,
                    store.preflight_activity_switch().unwrap_err().code,
                    store.control_usage().unwrap().objects,
                ))
                .unwrap();
        })))
        .unwrap();
    let client = service.client();
    assert!(!client.gate.close());
    release.send(()).unwrap();
    receive(&at_shutdown);
    let outcome = verified(late.job().cose()[0]);
    late.return_result(outcome).unwrap();
    fail_join.send(()).unwrap();
    assert_eq!(receive(&reports).unwrap_err().code, Code::Stopped);
    assert_eq!(receive(&checked), (2, Code::Stopped, 0));
    let error = service.stop().unwrap_err();
    assert_eq!(error.operation, "synthetic verifier Join failed");
    let source = error
        .source()
        .unwrap()
        .downcast_ref::<std::io::Error>()
        .unwrap();
    assert_eq!(source.raw_os_error(), Some(5));
    assert_eq!(client.status().unwrap_err().code, Code::Stopped);
    assert_eq!(
        Store::open(&workspace.dir, Limits::default(), clock(1000))
            .err()
            .unwrap()
            .code,
        Code::Busy
    );
    assert_eq!(
        still_connected
            .return_result(ControlVerificationOutcome::Rejected)
            .unwrap_err()
            .code,
        Code::Stopped
    );
}
