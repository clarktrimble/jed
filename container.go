package jed

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// Container is service container config.
// Todo: pub?
type Container struct {
	Name    string
	Image   string
	Ports   map[string]string `json:"ports,omitempty"`
	Labels  map[string]string `json:"labels,omitempty"`
	Volumes map[string]string `json:"volumes,omitempty"`
	Network string            `json:"network"`
	Restart string            `json:"restart"`
}

func (cntr *Container) validate() error {
	var issues []string

	if cntr.Image == "" {
		issues = append(issues, "image is required")
	}
	if cntr.Name == "" {
		issues = append(issues, "name is required")
	}
	//if cntr.EnvForm == "" {
	//issues = append(issues, "env_form is required")
	//}
	if cntr.Network == "" {
		issues = append(issues, "network is required")
	}
	if cntr.Restart == "" {
		issues = append(issues, "restart policy is required")
	}

	if len(issues) > 0 {
		return errors.Errorf("container %s invalid: %s", cntr.Name, strings.Join(issues, ", "))
	}

	return nil
}

type containerConfig map[string]any

func (cntr *Container) config() (cfg containerConfig, err error) {

	err = cntr.validate()
	// Todo: best effort valicate on load maybe?  But what if we change sommat?
	if err != nil {
		return
	}

	// Todo: elsewhere plz
	//err = svc.checkImage(ctx, cntr.Image)
	//if err != nil {
	//return
	//}

	exposedPorts, portBindings := buildPortConfig(cntr.Ports)

	// Todo: add managed_by=jed label when loading containers

	hostConfig := map[string]any{
		"PortBindings": portBindings,
		"Binds":        buildVolumeConfig(cntr.Volumes),
		"RestartPolicy": map[string]any{
			"Name": cntr.Restart,
		},
	}

	cfg = map[string]any{
		"Image": cntr.Image,
		//"Env":          envLines(cntr.Env),
		"Labels":       cntr.Labels,
		"ExposedPorts": exposedPorts,
		"HostConfig":   hostConfig,
		"NetworkingConfig": map[string]any{
			"EndpointsConfig": map[string]any{
				cntr.Network: map[string]any{},
			},
		},
	}

	return
}

func buildPortConfig(ports map[string]string) (exposedPorts map[string]any, portBindings map[string][]map[string]string) {
	exposedPorts = make(map[string]any)
	portBindings = make(map[string][]map[string]string)

	for containerPort, hostPort := range ports {
		exposedPorts[containerPort] = map[string]any{}
		portBindings[containerPort] = []map[string]string{
			{"HostPort": hostPort},
		}
	}

	return
}

func buildVolumeConfig(volumes map[string]string) []string {
	var binds []string
	for hostPath, containerPath := range volumes {
		binds = append(binds, fmt.Sprintf("%s:%s", hostPath, containerPath))
	}
	return binds
}
