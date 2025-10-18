package jed

import (
	"strings"

	"github.com/pkg/errors"
)

// Status is container status returned from docker
// Todo: rename? yes docker calls this a container, lol
type Status struct {
	Id      string            `json:"Id"`
	Names   []string          `json:"Names"`
	Image   string            `json:"Image"`
	State   string            `json:"State"`
	Status  string            `json:"Status"`
	Labels  map[string]string `json:"Labels"`
	Created int64             `json:"Created"`
}

// Statii maps container base names to their status.
type Statii map[string]Status

// DeployName returns the deployed name given a based name.
func (statii Statii) DeployName(baseName string) (deployName string, err error) {
	status, ok := statii[baseName]
	if !ok {
		err = errors.Errorf("container %s not found", baseName)
		return
	}
	deployName = strings.TrimPrefix(status.Names[0], "/")
	return
}

// Id returns the container ID given a base name.
func (statii Statii) Id(baseName string) (id string, err error) {
	status, ok := statii[baseName]
	if !ok {
		err = errors.Errorf("container %s not found", baseName)
		return
	}
	id = status.Id
	return
}

// unexported

func newStatii(statuses []Status) (statii Statii) {

	statii = Statii{}
	for _, status := range statuses {
		for _, name := range status.Names {
			fullName := strings.TrimPrefix(name, "/")
			baseName := suffixPattern.ReplaceAllString(fullName, "")

			statii[baseName] = status
		}
	}
	return statii
}
