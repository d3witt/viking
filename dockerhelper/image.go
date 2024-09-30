package dockerhelper

import (
	"context"
	"fmt"
	"io"
	"log/slog"

	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
)

func DistributeImage(ctx context.Context, local *client.Client, remote *client.Client, imageName string) error {
	slog.InfoContext(ctx, "Distributing image", "image", imageName)

	inspect, _, err := local.ImageInspectWithRaw(ctx, imageName)
	if err != nil {
		return fmt.Errorf("failed to inspect image on local client: %w", err)
	}

	remoteImages, err := remote.ImageList(ctx, image.ListOptions{})
	if err != nil {
		return fmt.Errorf("failed to list images on remote client: %w", err)
	}

	for _, img := range remoteImages {
		if img.ID == inspect.ID {
			slog.InfoContext(ctx, "Image already exists on remote client", "image", imageName)
			return nil
		}
	}

	reader, err := local.ImageSave(ctx, []string{imageName})
	if err != nil {
		return fmt.Errorf("failed to save image from local client: %w", err)
	}
	defer reader.Close()

	resp, err := remote.ImageLoad(ctx, reader, true)
	if err != nil {
		return fmt.Errorf("failed to load image on remote client: %w", err)
	}
	defer resp.Body.Close()

	_, err = io.Copy(io.Discard, resp.Body)
	if err != nil {
		return fmt.Errorf("error reading response from remote client: %w", err)
	}

	return nil
}
