package dockerhelper

import (
	"fmt"
	"strings"

	"github.com/d3witt/viking/sshexec"
)

var traefikImage = "traefik:2.11"

func getTraefikArgs(certEmail string) ([]string, error) {
	return []string{
		"--providers.docker",
		"--providers.docker.exposedByDefault=false",
		"--log.level=DEBUG",
		"--entrypoints.web.address=:80",
		"--entrypoints.web.http.redirections.entryPoint.to=websecure",
		"--entrypoints.web.http.redirections.entryPoint.scheme=https",
		"--entrypoints.websecure.address=:443",
		fmt.Sprintf("--certificatesresolvers.myresolver.acme.email=%s", certEmail),
		"--certificatesresolvers.myresolver.acme.storage=./acme.json",
		"--certificatesresolvers.myresolver.acme.tlschallenge=true",
		"--accesslog=true",
	}, nil
}

var traefikVolumes = []string{
	"/var/run/docker.sock:/var/run/docker.sock",
}

var traefikPorts = []string{
	"80:80",
	"443:443",
}

var (
	traefikName   = "viking-traefik"
	vikingNetwork = "viking-network"
)

func runTraefik(e sshexec.Executor, certEmail string) error {
	// traefikArgs, err := getTraefikArgs(certEmail)
	// if err != nil {
	// 	return err
	// }

	// return RunContainer(client, traefikImage, traefikName, traefikPorts, traefikVolumes, nil, nil, traefikArgs...)
	return nil
}

func GetTraefikRouteLabel(domain, containerPort string) []string {
	domainLabel := strings.ReplaceAll(domain, ".", "-")
	return []string{
		"traefik.docker.network=" + vikingNetwork,
		"traefik.enable=true",
		fmt.Sprintf("traefik.http.routers.%s.rule=Host(`%s`)", domainLabel, domain),
		fmt.Sprintf("traefik.http.routers.%s.entrypoints=websecure", domainLabel),
		fmt.Sprintf("traefik.http.routers.%s.tls=true", domainLabel),
		fmt.Sprintf("traefik.http.routers.%s.tls.certresolver=myresolver", domainLabel),
		fmt.Sprintf("traefik.http.services.%s.loadbalancer.server.port=%s", domainLabel, containerPort),
	}
}
