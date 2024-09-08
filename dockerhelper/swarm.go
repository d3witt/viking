package dockerhelper

import (
	"context"
	"errors"

	"github.com/docker/docker/api/types/swarm"
)

var ErrNoManagerFound = errors.New("no manager node found or available")

func ManagerNode(ctx context.Context, clients []*Client) (*Client, error) {
	if len(clients) == 0 {
		return nil, ErrNoManagerFound
	}

	type result struct {
		client *Client
		err    error
	}

	resultCh := make(chan result, len(clients))

	for _, item := range clients {
		go func(cl *Client) {
			info, err := cl.Info(ctx)
			if err != nil {
				resultCh <- result{err: err}
				return
			}

			if info.Swarm.LocalNodeState == swarm.LocalNodeStateActive && info.Swarm.ControlAvailable {
				resultCh <- result{client: cl}
			} else {
				resultCh <- result{err: ErrNoManagerFound}
			}
		}(item)
	}

	for i := 0; i < len(clients); i++ {
		select {
		case res := <-resultCh:
			if res.err == nil {
				return res.client, nil
			}
		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}

	return nil, ErrNoManagerFound
}
