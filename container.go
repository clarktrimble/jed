package jed

import (
	"strings"

	"github.com/pkg/errors"
)

// Container represents a Docker container as returned by the Docker API.
type Container struct {
	Id      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
	// Service is the service name parsed from the first container name.
	Service string `json:"-"`
}

// DeployName returns the container's deployed name with leading slash removed.
func (c Container) DeployName() string {

	if len(c.Names) == 0 {
		return "container has no names, most unexpected"
	}

	return strings.TrimPrefix(c.Names[0], "/")
}

// Containers is a slice of containers.
type Containers []Container

// Find finds a container given a service name.
func (ctrs Containers) Find(serviceName string) (ctr Container, err error) {

	for _, ctr = range ctrs {
		if ctr.Service == serviceName {
			return
		}
	}
	err = errors.Errorf("container %s not found", serviceName)
	return
}

// unexported

// Todo: Find vs deployed, wrong somewhere?
// Todo: is more flexible to accept slice?
func (ctrs Containers) deployed(name string) bool {

	for _, ctr := range ctrs {
		if ctr.Service == name {
			return true
		}
	}
	return false
}

func managed(ctrs Containers) (mgd Containers, noMatch []string) {

	mgd = Containers{}
	for _, ctr := range ctrs {
		for _, name := range ctr.Names {

			match := suffixPattern.FindStringSubmatch(name)
			if match == nil {
				noMatch = append(noMatch, name)
				continue
			}

			ctr.Service = match[1]
			mgd = append(mgd, ctr)
		}
	}
	return
}
