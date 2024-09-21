package dockerhelper

import (
	"context"
	"time"
)

func WaitFor(
	ctx context.Context,
	timeout time.Duration,
	interval time.Duration,
	checkFn func(context.Context) (bool, error),
) error {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		done, err := checkFn(ctx)
		if err != nil {
			return err
		}
		if done {
			return nil
		}

		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(interval):
		}
	}
}
