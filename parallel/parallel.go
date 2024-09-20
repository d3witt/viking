package parallel

import (
	"context"
	"sync"
)

func ForEach(ctx context.Context, count int, task func(int)) {
	var wg sync.WaitGroup
	wg.Add(count)

	for i := 0; i < count; i++ {
		go func(i int) {
			defer wg.Done()
			task(i)
		}(i)
	}

	wg.Wait()
}

func RunFirstErr(ctx context.Context, count int, task func(int) error) error {
	errc := make(chan error, 1)
	var wg sync.WaitGroup
	for i := 0; i < count; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			if err := task(i); err != nil {
				select {
				case errc <- err:
				default:
				}
			}
		}(i)
	}

	go func() {
		wg.Wait()
		close(errc)
	}()

	select {
	case err := <-errc:
		wg.Wait()
		return err
	case <-ctx.Done():
		wg.Wait()
		return ctx.Err()
	}
}
