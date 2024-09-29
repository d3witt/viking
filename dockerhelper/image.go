package dockerhelper

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"sync"
)

func DistributeImage(ctx context.Context, imageReader io.ReadCloser, targetClients []*Client, imageName string) error {
	slog.InfoContext(ctx, "Distributing image", "image", imageName)

	pr, pw := io.Pipe()
	go func() {
		defer pw.Close()
		_, err := io.Copy(pw, imageReader)
		if err != nil {
			pw.CloseWithError(err)
		}
	}()

	var wg sync.WaitGroup
	errChan := make(chan error, len(targetClients))

	for _, client := range targetClients {
		wg.Add(1)
		go func(c *Client) {
			defer wg.Done()

			// Create a new reader for each client
			pipeReader, pipeWriter := io.Pipe()
			go func() {
				defer pipeWriter.Close()
				_, err := io.Copy(pipeWriter, pr)
				if err != nil {
					pipeWriter.CloseWithError(err)
				}
			}()

			// Load the image on the target client
			resp, err := c.ImageLoad(ctx, pipeReader, true)
			if err != nil {
				errChan <- fmt.Errorf("failed to load image on %s: %w", c.RemoteHost(), err)
				return
			}
			defer resp.Body.Close()

			// Read and discard the response to ensure the operation completes
			_, err = io.Copy(io.Discard, resp.Body)
			if err != nil {
				errChan <- fmt.Errorf("error reading response from %s: %w", c.RemoteHost(), err)
			}
		}(client)
	}

	wg.Wait()
	close(errChan)

	// Collect any errors
	var errors []error
	for err := range errChan {
		errors = append(errors, err)
	}

	if len(errors) > 0 {
		return fmt.Errorf("errors occurred during image distribution: %v", errors)
	}

	return nil
}
