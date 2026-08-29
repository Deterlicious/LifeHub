package reminders

import (
	"context"
	"testing"

	"github.com/riverqueue/river/rivertype"
)

type failureRecorderFake struct {
	jobIDs []int64
	err    error
}

func (fake *failureRecorderFake) InvalidateReminderJob(_ context.Context, jobID int64) error {
	fake.jobIDs = append(fake.jobIDs, jobID)
	return fake.err
}

func TestErrorHandlerOnlyInvalidatesTerminalReminderAttempts(t *testing.T) {
	recorder := &failureRecorderFake{}
	handler := NewErrorHandler(recorder, nil)
	handler.HandleError(context.Background(), &rivertype.JobRow{ID: 1, Kind: JobKind, Attempt: MaxAttempts - 1, MaxAttempts: MaxAttempts}, nil)
	handler.HandleError(context.Background(), &rivertype.JobRow{ID: 2, Kind: "other", Attempt: MaxAttempts, MaxAttempts: MaxAttempts}, nil)
	if len(recorder.jobIDs) != 0 {
		t.Fatalf("non-terminal jobs invalidated: %v", recorder.jobIDs)
	}
	handler.HandleError(context.Background(), &rivertype.JobRow{ID: 3, Kind: JobKind, Attempt: MaxAttempts, MaxAttempts: MaxAttempts}, nil)
	if len(recorder.jobIDs) != 1 || recorder.jobIDs[0] != 3 {
		t.Fatalf("terminal jobs = %v, want [3]", recorder.jobIDs)
	}
}
