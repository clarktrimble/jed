package jed

import (
	"strings"

	"github.com/pkg/errors"
)

// Container is container status returned from docker
type Container struct {
	Id      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
}

// Containers maps service names to their runtime container status.
type Containers map[string]Container

// DeployName returns the deployed name given a service name.
func (containers Containers) DeployName(serviceName string) (deployName string, err error) {
	container, ok := containers[serviceName]
	if !ok {
		err = errors.Errorf("container %s not found", serviceName)
		return
	}
	deployName = strings.TrimPrefix(container.Names[0], "/")
	return
}

// Id returns the container ID given a service name.
func (containers Containers) Id(serviceName string) (id string, err error) {
	container, ok := containers[serviceName]
	if !ok {
		err = errors.Errorf("container %s not found", serviceName)
		return
	}
	id = container.Id
	return
}

// unexported

func newContainers(containers []Container) (result Containers) {

	result = Containers{}
	for _, container := range containers {
		for _, name := range container.Names {
			fullName := strings.TrimPrefix(name, "/")
			serviceName := suffixPattern.ReplaceAllString(fullName, "")

			result[serviceName] = container
		}
	}
	return result
}
