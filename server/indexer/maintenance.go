package indexer

import (
	"context"
	"errors"
	"fmt"
	"sync"

	"github.com/rs/zerolog/log"
)

type maintenanceTask struct {
	name string
	run  func(context.Context) error
}

type maintenanceResult struct {
	name string
	err  error
}

// maintenanceRunner executes compatibility work sequentially without holding
// up indexer initialization.
type maintenanceRunner struct {
	ctx       context.Context
	cancel    context.CancelFunc
	startOnce sync.Once
	wg        sync.WaitGroup
	resultsMu sync.Mutex
	results   []maintenanceResult
}

func newMaintenanceRunner() *maintenanceRunner {
	ctx, cancel := context.WithCancel(context.Background())
	return &maintenanceRunner{
		ctx:    ctx,
		cancel: cancel,
	}
}

func (r *maintenanceRunner) start(tasks ...maintenanceTask) {
	r.startOnce.Do(func() {
		registered := append([]maintenanceTask(nil), tasks...)
		r.wg.Go(func() {
			for _, task := range registered {
				if r.ctx.Err() != nil {
					return
				}
				err := task.run(r.ctx)
				r.resultsMu.Lock()
				r.results = append(r.results, maintenanceResult{name: task.name, err: err})
				r.resultsMu.Unlock()
				if err != nil && r.ctx.Err() == nil {
					log.Error().Err(err).Str("task", task.name).Msg("Background maintenance task failed")
				}
			}
		})
	})
}

func (r *maintenanceRunner) stop() {
	r.cancel()
}

func (r *maintenanceRunner) wait() error {
	r.wg.Wait()
	r.resultsMu.Lock()
	defer r.resultsMu.Unlock()
	errs := make([]error, 0, len(r.results))
	for _, result := range r.results {
		if result.err != nil {
			errs = append(errs, fmt.Errorf("%s: %w", result.name, result.err))
		}
	}
	return errors.Join(errs...)
}
