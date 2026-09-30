package queue

import (
	"errors"
	"testing"
	"time"

	"github.com/hibiken/asynq"
)

// Report deliveries retry slowly enough for greylisting mail servers to let
// the second attempt through (typical greylist windows are 5 to 15 minutes);
// asynq's default (well under a minute for the first retries) is too fast.
func TestScheduledReportRetryDelayOutwaitsGreylisting(t *testing.T) {
	task := asynq.NewTask(TypeScheduledReportRun, nil)
	err := errors.New("smtp: 450 greylisted")
	want := []time.Duration{10 * time.Minute, 20 * time.Minute, 40 * time.Minute}
	for retried, w := range want {
		if got := retryDelay(retried, err, task); got != w {
			t.Fatalf("retried=%d: got %s, want %s", retried, got, w)
		}
	}
	if got := retryDelay(9, err, task); got != 40*time.Minute {
		t.Fatalf("later retries must keep the last step, got %s", got)
	}
}

func TestRetryDelayKeepsAsynqDefaultForOtherTasks(t *testing.T) {
	task := asynq.NewTask(TypeRunPatch, nil)
	err := errors.New("boom")
	if got := retryDelay(0, err, task); got >= 5*time.Minute {
		t.Fatalf("other task types must keep asynq's fast default, got %s", got)
	}
}
