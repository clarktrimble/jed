package container

import (
	"regexp"
	"slices"
	"strings"

	"github.com/pkg/errors"
)

var deployNamePattern = regexp.MustCompile(`^/(.+)-[a-zA-Z0-9]{7}$`)

// Container represents a Docker container as returned by the Docker API.
type Container struct {
	Id      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
	// ServiceName is parsed from first of Names to hopefully match Service.Name.
	ServiceName string `json:"-"`
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
		if ctr.ServiceName == serviceName {
			return
		}
	}
	err = errors.Errorf("container %s not found", serviceName)
	return
}

// unexported

func (ctrs Containers) deployed(name string) bool {

	return slices.ContainsFunc(ctrs, func(c Container) bool {
		return c.ServiceName == name
	})
}

func (ctrs Containers) managed() (mgd Containers, noMatch []string) {

	mgd = Containers{}
	for _, ctr := range ctrs {
		for _, name := range ctr.Names {

			match := deployNamePattern.FindStringSubmatch(name)
			if match == nil {
				noMatch = append(noMatch, name)
				continue
			}

			ctr.ServiceName = match[1]
			mgd = append(mgd, ctr)
		}
	}
	return
}
