package utils

import (
	"fmt"
	"os"
	"runtime"

	dockerclient "github.com/docker/docker/client"
)

var (
	// DefaultHTTPHost Default HTTP Host
	DefaultHTTPHost = "localhost"
	// DefaultHTTPPort Default HTTP Port
	DefaultHTTPPort = 2375
	// DefaultUnixSocket Path for the unix socket.
	DefaultUnixSocket = "/var/run/docker.sock"
)

// getDockerHost returns the docker socket based on Environment settings
func getDockerHost() string {
	dockerHost := os.Getenv("DOCKER_HOST")
	if dockerHost == "" {
		if runtime.GOOS == "windows" {
			// If we do not have a host, default to TCP socket on Windows
			dockerHost = fmt.Sprintf("tcp://%s:%d", DefaultHTTPHost, DefaultHTTPPort)
		} else {
			// If we do not have a host, default to unix socket
			dockerHost = fmt.Sprintf("unix://%s", DefaultUnixSocket)
		}
	}
	return dockerHost
}

// GetDockerClient returns a new Docker Client based on the environment settings
func GetDockerClient() (*dockerclient.Client, error) {
	return dockerclient.NewClient(getDockerHost(), "", nil, nil)
}
