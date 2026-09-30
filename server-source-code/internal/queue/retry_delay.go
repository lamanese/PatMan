package queue

import (
	"time"

	"github.com/hibiken/asynq"
)

// scheduledReportRetryDelays spaces the retries of a report run so that a
// greylisting mail server (typically 5 to 15 minutes) accepts the retry; the
// last step repeats for any further attempt.
var scheduledReportRetryDelays = []time.Duration{10 * time.Minute, 20 * time.Minute, 40 * time.Minute}

// retryDelay is the asynq RetryDelayFunc: report runs use the slow schedule
// above, every other task keeps asynq's default backoff. retried is the
// number of retries already made (0 on the first failure).
func retryDelay(retried int, err error, task *asynq.Task) time.Duration {
	if task.Type() != TypeScheduledReportRun {
		return asynq.DefaultRetryDelayFunc(retried, err, task)
	}
	if retried < 0 {
		retried = 0
	}
	if retried >= len(scheduledReportRetryDelays) {
		retried = len(scheduledReportRetryDelays) - 1
	}
	return scheduledReportRetryDelays[retried]
}
