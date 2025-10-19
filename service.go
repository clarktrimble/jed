package jed

import (
	"fmt"
	"strings"

	"github.com/pkg/errors"
)

// Service is a service configuration.
type Service struct {
	Name    string
	Image   string
	Env     map[string]string `json:"env"`
	Ports   map[string]string `json:"ports"`
	Labels  map[string]string `json:"labels"`
	Volumes map[string]string `json:"volumes"`
	Network string            `json:"network"`
	Restart string            `json:"restart"`
}

func (service Service) validate() error {
	var issues []string

	if service.Image == "" {
		issues = append(issues, "image is required")
	}
	if service.Name == "" {
		issues = append(issues, "name is required")
	}
	if service.Network == "" {
		issues = append(issues, "network is required")
	}
	if service.Restart == "" {
		issues = append(issues, "restart policy is required")
	}

	if len(issues) > 0 {
		return errors.Errorf("container %s invalid: %s", service.Name, strings.Join(issues, ", "))
	}

	return nil
}

type containerConfig map[string]any

func (service Service) config() (cfg containerConfig, err error) {

	exposedPorts, portBindings := buildPortConfig(service.Ports)

	hostConfig := map[string]any{
		"PortBindings": portBindings,
		"Binds":        buildVolumeConfig(service.Volumes),
		"RestartPolicy": map[string]any{
			"Name": service.Restart,
		},
	}

	cfg = map[string]any{
		"Image":        service.Image,
		"Env":          envLines(service.Env),
		"Labels":       service.Labels,
		"ExposedPorts": exposedPorts,
		"HostConfig":   hostConfig,
		"NetworkingConfig": map[string]any{
			"EndpointsConfig": map[string]any{
				service.Network: map[string]any{},
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

func envLines(env map[string]string) (lines []string) {

	// godotenv.Marshal mangled the vals, maybe with quotes? anyway ..

	for key, value := range env {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return
}
