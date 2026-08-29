package main

import (
	"context"
	"errors"
	"testing"
	"time"
)

type blockingStopper struct {
	hardCalled bool
}

func (stopper *blockingStopper) Stop(ctx context.Context) error {
	<-ctx.Done()
	return ctx.Err()
}

func (stopper *blockingStopper) StopAndCancel(ctx context.Context) error {
	stopper.hardCalled = true
	if _, ok := ctx.Deadline(); !ok {
		return errors.New("hard stop context has no deadline")
	}
	return nil
}

func TestStopWorkerBoundsHardCancellation(t *testing.T) {
	stopper := &blockingStopper{}
	started := time.Now()
	err := stopWorker(stopper, 10*time.Millisecond, 10*time.Millisecond)
	if err == nil || !stopper.hardCalled {
		t.Fatalf("err=%v hard_called=%v", err, stopper.hardCalled)
	}
	if elapsed := time.Since(started); elapsed > time.Second {
		t.Fatalf("bounded shutdown took %s", elapsed)
	}
}
