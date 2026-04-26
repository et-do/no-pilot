package execute

import "sync"

type lastTestRun struct {
	Ran           bool
	Failed        bool
	Summary       string
	FailureDetail string
}

var (
	lastTestRunMu sync.RWMutex
	lastRun       lastTestRun
)

func setLastTestRun(state lastTestRun) {
	lastTestRunMu.Lock()
	lastRun = state
	lastTestRunMu.Unlock()
}

func getLastTestRun() lastTestRun {
	lastTestRunMu.RLock()
	defer lastTestRunMu.RUnlock()
	return lastRun
}
