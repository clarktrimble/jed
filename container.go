package jed

import (
	"strings"

	"github.com/pkg/errors"
)

// Container is from docker api.
type Container struct {
	Id      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
	Service string            `json:"-"`
}

// DeployName returns the container's deployed name with leading slash removed.
func (c Container) DeployName() string {
	return strings.TrimPrefix(c.Names[0], "/")
}

// Containers is a slice of containers.
type Containers []Container

// Find returns the container for a given service name.
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

func managed(ctrs []Container) (mgd Containers, noMatch []string) {

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
