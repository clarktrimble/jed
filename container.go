package jed

import (
	"fmt"
	"io/fs"
	"strings"

	"github.com/joho/godotenv"
	"github.com/pkg/errors"
	"sigs.k8s.io/yaml"
)

// Container is service Container config.
type Container struct {
	Name    string
	Image   string
	Env     map[string]string `json:"env"`
	Ports   map[string]string `json:"ports"` //,omitempty" Todo:
	Labels  map[string]string `json:"labels"`
	Volumes map[string]string `json:"volumes"` //,omitempty" Todo:
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

	exposedPorts, portBindings := buildPortConfig(cntr.Ports)

	cntr.Labels["managed_by"] = "jed"

	hostConfig := map[string]any{
		"PortBindings": portBindings,
		"Binds":        buildVolumeConfig(cntr.Volumes),
		"RestartPolicy": map[string]any{
			"Name": cntr.Restart,
		},
	}

	cfg = map[string]any{
		"Image":        cntr.Image,
		"Env":          envLines(cntr.Env),
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

func loadContainers(cfs fs.FS) (containers []Container, err error) {

	// Todo: hardcoded filename is awkward here, rethink
	data, err := fs.ReadFile(cfs, "containers.yaml")
	if err != nil {
		err = errors.Wrap(err, "failed to read containers.yaml")
		return
	}

	err = yaml.Unmarshal(data, &containers)
	if err != nil {
		err = errors.Wrapf(err, "failed to decode containers config")
		return
	}

	for i, container := range containers {

		err = container.validate()
		if err != nil {
			return
		}

		var env map[string]string
		env, err = loadEnv(cfs, container.Name)
		if err != nil {
			return
		}
		containers[i].Env = env
	}

	return
}

func loadEnv(cfs fs.FS, name string) (env map[string]string, err error) {

	// Todo: demajic
	file := fmt.Sprintf("%s.env", name)

	env = map[string]string{}
	envData, err := fs.ReadFile(cfs, file)
	if errors.Is(err, fs.ErrNotExist) {
		err = nil
		return
	}
	if err != nil {
		err = errors.Wrapf(err, "cannot read %s", file)
		return
	}

	env, err = godotenv.Unmarshal(string(envData))
	err = errors.Wrapf(err, "cannot unmarshal %s", file)
	return
}

func envLines(env map[string]string) (lines []string) {

	// godotenv.Marshal mangled the vals, maybe with quotes? anyway ..

	for key, value := range env {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return
}
