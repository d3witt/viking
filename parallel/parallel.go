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
