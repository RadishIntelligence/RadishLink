//! Owner-side scheduling of prepared control verification input.
use super::*;
use crate::store::{
    ControlNeighbor, ControlVerificationCompletion, ControlVerificationDispatch,
    OutboundAckAdmission, OwnerShutdownWork,
};

type Completion = (ControlVerificationCompletion, OutboundAckAdmission);

/// Internal source for already prepared inputs. Dispatch and reporting must be bounded and
/// nonblocking. The shutdown hook records closure of the supplied instances and returns its
/// Join result; a task can return while that hook runs, using its original result slot.
pub(super) trait VerificationDriver: Send {
    fn neighbors(&self) -> &[ControlNeighbor];
    fn dispatch(&mut self, task: ControlVerificationDispatch);
    fn report(&mut self, result: Result<Completion>);
    fn stop_and_join(&mut self, store: &mut Store, work: OwnerShutdownWork) -> Result<()>;
}

pub(super) fn collect(store: &mut Store, driver: &mut dyn VerificationDriver) -> bool {
    let result = match store.poll_control_result(driver.neighbors(), true) {
        Ok(None) => return false,
        Ok(Some(done)) => Ok(done),
        Err(error) => Err(error),
    };
    driver.report(result);
    true
}

pub(super) fn dispatch(
    store: &mut Store,
    driver: &mut dyn VerificationDriver,
    wake: &SyncSender<()>,
) -> bool {
    if store.is_quarantined()
        || store.is_time_uncertain()
        || store.control_ingress_usage().waiting == 0
    {
        return false;
    }
    match store.dispatch_control_with_wake(Some(wake.clone())) {
        Ok(poll) => {
            for error in poll.discarded {
                driver.report(Err(error));
            }
            if let Some(task) = poll.dispatch {
                driver.dispatch(task);
                return true;
            }
        }
        Err(error) => driver.report(Err(error)),
    }
    false
}

pub(super) fn shutdown(store: &mut Store, driver: &mut dyn VerificationDriver) -> Result<()> {
    let work = store.begin_owner_shutdown();
    let joined = driver.stop_and_join(store, work);
    // Closing admission bounds this drain by the original leases. Collect actual returns even
    // if Join failed. Abandoned tasks can be reclaimed only by a confirmed shutdown.
    while collect(store, driver) {}
    store.finish_owner_shutdown(joined)
}

#[cfg(test)]
mod tests;
