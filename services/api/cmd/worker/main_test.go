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

func TestParseRuntimeModeDefaultsToContinuousPeriodicWorker(t *testing.T) {
	mode, err := parseRuntimeMode("", "")
	if err != nil {
		t.Fatal(err)
	}
	if mode.runDuration != 0 || !mode.periodicJobs {
		t.Fatalf("mode=%+v", mode)
	}
}

func TestParseRuntimeModeSupportsBoundedReminderOnlyJob(t *testing.T) {
	mode, err := parseRuntimeMode("45s", "false")
	if err != nil {
		t.Fatal(err)
	}
	if mode.runDuration != 45*time.Second || mode.periodicJobs {
		t.Fatalf("mode=%+v", mode)
	}
}

func TestParseRuntimeModeRejectsUnsafeValues(t *testing.T) {
	tests := []struct {
		name     string
		duration string
		periodic string
	}{
		{name: "malformed duration", duration: "soon"},
		{name: "too short", duration: "1s"},
		{name: "too long", duration: "10m"},
		{name: "invalid periodic flag", periodic: "sometimes"},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := parseRuntimeMode(test.duration, test.periodic); err == nil {
				t.Fatal("expected validation error")
			}
		})
	}
}
