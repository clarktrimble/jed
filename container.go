package jed

import (
	"strings"

	"github.com/pkg/errors"
)

// Container is docker api container.
type Container struct {
	Id      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
}

// Containers maps service names to container.
type Containers map[string]Container

// DeployName returns the deployed name of a given service.
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

func newContainers(containers []Container) (byName Containers, noMatch []string) {

	byName = Containers{}
	for _, container := range containers {
		for _, name := range container.Names {
			matches := suffixPattern.FindStringSubmatch(name)
			if matches == nil {
				noMatch = append(noMatch, name)
				continue
			}

			serviceName := matches[1]
			byName[serviceName] = container
		}
	}
	return
}
