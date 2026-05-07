package container

import (
	"fmt"
	"maps"

	"github.com/clarktrimble/jed"
)

type containerConfig map[string]any

func buildConfig(service jed.Service, env jed.Env) (cfg containerConfig, err error) {
	exposedPorts, portBindings := buildPortConfig(service.Ports)

	hostConfig := map[string]any{
		"PortBindings":  portBindings,
		"Binds":         buildVolumeConfig(service.Volumes),
		"RestartPolicy": containerRestartPolicy(service.Restart),
	}

	cfg = map[string]any{
		"Image":        service.Image,
		"Env":          envLines(env.Vars),
		"Labels":       managedBy(service.Labels),
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

func managedBy(labels map[string]string) map[string]string {
	withLabel := map[string]string{}
	maps.Copy(withLabel, labels)
	withLabel["managed_by"] = "jed"

	return withLabel
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

func containerRestartPolicy(name string) map[string]any {
	switch name {
	case "", jed.RestartNone:
		name = "no"
	case jed.RestartAny:
		name = "always"
	}

	return map[string]any{"Name": name}
}

func envLines(env map[string]string) (lines []string) {
	for key, value := range env {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}
	return
}
