package machine

import "github.com/d3witt/viking/dockerhelper"

func closeDockerClients(clients []*dockerhelper.Client) {
	for _, client := range clients {
		if client != nil {
			client.Close()
		}
	}
}
