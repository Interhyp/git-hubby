package reconciler

import "errors"

// StopReconciliation can be returned by a Reconciliation to stop the run on purpose because the desired final state
// has been reached and the remaining reconciliations must not run (e.g. the repository is archived and therefore
// read-only). It is not treated as a failure:
//   - the condition of the returning reconciliation is set to True with Message,
//   - the conditions of all reconciliations in later groups are set to Unknown (reason Skipped),
//   - the run is successful (Ready=True, observed sub-resource generations are updated) unless another
//     reconciliation of the same group failed.
type StopReconciliation struct {
	// Message describes why the reconciliation was stopped and is used for all affected conditions.
	Message string
}

func (s *StopReconciliation) Error() string {
	return "reconciliation stopped: " + s.Message
}

// asStopReconciliation returns the StopReconciliation contained in err, if any.
func asStopReconciliation(err error) (*StopReconciliation, bool) {
	if err == nil {
		return nil, false
	}
	return errors.AsType[*StopReconciliation](err)
}
