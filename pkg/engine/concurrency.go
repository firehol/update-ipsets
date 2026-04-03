package engine

import (
	"context"
	"errors"
	"sync"
)

func nonNilContext(ctx context.Context) context.Context {
	if ctx == nil {
		return context.Background()
	}
	return ctx
}

func contextErr(ctx context.Context) error {
	if ctx == nil {
		return nil
	}
	return ctx.Err()
}

type joinedErrorCollector struct {
	mu   sync.Mutex
	errs []error
}

func (c *joinedErrorCollector) add(err error) bool {
	if err == nil {
		return false
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	first := len(c.errs) == 0
	c.errs = append(c.errs, err)
	return first
}

func (c *joinedErrorCollector) hasErrors() bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	return len(c.errs) > 0
}

func (c *joinedErrorCollector) err() error {
	c.mu.Lock()
	defer c.mu.Unlock()
	return errors.Join(c.errs...)
}

func runBoundedNameJobs(ctx context.Context, workers int, names []string, fn func(context.Context, string) error) error {
	ctx = nonNilContext(ctx)
	if err := contextErr(ctx); err != nil {
		return err
	}
	if len(names) == 0 {
		return nil
	}
	if workers < 1 {
		workers = 1
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan string)
	var wg sync.WaitGroup
	var errs joinedErrorCollector
	setErr := func(err error) {
		if errs.add(err) {
			cancel()
		}
	}

	for range workers {
		wg.Go(func() {
			for {
				select {
				case <-ctx.Done():
					return
				case name, ok := <-jobs:
					if !ok {
						return
					}
					if err := fn(ctx, name); err != nil {
						setErr(err)
						return
					}
				}
			}
		})
	}

send:
	for _, name := range names {
		select {
		case <-ctx.Done():
			break send
		case jobs <- name:
		}
	}
	close(jobs)
	wg.Wait()

	if err := errs.err(); err != nil {
		return err
	}
	return contextErr(ctx)
}
