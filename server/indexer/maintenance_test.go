package indexer

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

func TestMaintenanceRunnerRunsTasksSequentially(t *testing.T) {
	runner := newMaintenanceRunner()
	order := []string{}
	runner.start(
		maintenanceTask{name: "first", run: func(context.Context) error {
			order = append(order, "first")
			return nil
		}},
		maintenanceTask{name: "second", run: func(context.Context) error {
			order = append(order, "second")
			return nil
		}},
	)

	if err := runner.wait(); err != nil {
		t.Fatalf("maintenance failed: %v", err)
	}
	if want := []string{"first", "second"}; !reflect.DeepEqual(order, want) {
		t.Fatalf("task order = %v, want %v", order, want)
	}
}

func TestMaintenanceRunnerCollectsErrorsAndContinues(t *testing.T) {
	runner := newMaintenanceRunner()
	secondRan := false
	runner.start(
		maintenanceTask{name: "broken", run: func(context.Context) error {
			return errors.New("failure")
		}},
		maintenanceTask{name: "second", run: func(context.Context) error {
			secondRan = true
			return nil
		}},
	)

	err := runner.wait()
	if err == nil || !strings.Contains(err.Error(), "broken: failure") {
		t.Fatalf("maintenance error = %v, want named task error", err)
	}
	if !secondRan {
		t.Fatal("task after a failed task did not run")
	}
}

func TestMaintenanceRunnerStopsBeforeNextTask(t *testing.T) {
	runner := newMaintenanceRunner()
	started := make(chan struct{})
	secondRan := false
	runner.start(
		maintenanceTask{name: "blocking", run: func(ctx context.Context) error {
			close(started)
			<-ctx.Done()
			return ctx.Err()
		}},
		maintenanceTask{name: "second", run: func(context.Context) error {
			secondRan = true
			return nil
		}},
	)
	<-started
	runner.stop()

	if err := runner.wait(); !errors.Is(err, context.Canceled) {
		t.Fatalf("maintenance error = %v, want context.Canceled", err)
	}
	if secondRan {
		t.Fatal("task ran after maintenance was stopped")
	}
}
