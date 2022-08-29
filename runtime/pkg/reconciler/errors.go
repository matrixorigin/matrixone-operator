package reconciler

import (
	"fmt"
	"time"
)

type ReSync struct {
	Message      string
	RequeueAfter time.Duration
}

func (e *ReSync) Error() string {
	return fmt.Sprintf("reconcile error: %s, requeue after %s", e.Message, e.RequeueAfter)
}

func ErrReSync(msg string, requeueAfter ...time.Duration) *ReSync {
	e := &ReSync{
		Message: msg,
	}
	if len(requeueAfter) > 0 {
		e.RequeueAfter = requeueAfter[0]
	}
	return e
}
