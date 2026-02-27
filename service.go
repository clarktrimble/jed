package jed

import (
	"context"
	"fmt"
	"maps"
	"strings"

	"github.com/pkg/errors"
)

// Resource defaults
const (
	DefaultCPULimit   = "0.5"
	DefaultMemLimit   = "128M"
	DefaultCPUReserve = "0.1"
	DefaultMemReserve = "64M"
)

// Resources specifies CPU and memory limits and reservations.
// CPU values are decimal strings (e.g., "0.5" for half a CPU).
// Memory values require M suffix (e.g., "128M" for 128 megabytes).
type Resources struct {
	CPULimit   string `json:"cpu_limit,omitempty"`
	MemLimit   string `json:"mem_limit,omitempty"`
	CPUReserve string `json:"cpu_reserve,omitempty"`
	MemReserve string `json:"mem_reserve,omitempty"`
}

// WithDefaults returns Resources with defaults applied for empty values.
func (r Resources) WithDefaults() Resources {
	if r.CPULimit == "" {
		r.CPULimit = DefaultCPULimit
	}
	if r.MemLimit == "" {
		r.MemLimit = DefaultMemLimit
	}
	if r.CPUReserve == "" {
		r.CPUReserve = DefaultCPUReserve
	}
	if r.MemReserve == "" {
		r.MemReserve = DefaultMemReserve
	}
	return r
}

// Service is a service's configuration.
type Service struct {
	// Name is the service name (e.g., "postgres").
	Name string
	// Image is the Docker image (e.g., "postgres:16").
	Image string
	// Command overrides the image's default command (e.g., ["sleep", "3600"]).
	Command []string `json:"command,omitempty"`
	// Ports maps container ports to host ports (e.g., "5432/tcp": "5432").
	Ports map[string]string `json:"ports"`
	// Labels are Docker container labels.
	Labels map[string]string `json:"labels"`
	// Volumes maps host paths to container paths.
	Volumes map[string]string `json:"volumes"`
	// Network is the Docker network name.
	Network string `json:"network"`
	// Restart is the restart policy (e.g., "unless-stopped", "always").
	Restart string `json:"restart"`
	// Secrets lists swarm secret base names to mount (e.g., "s3_secret_key").
	Secrets []string `json:"secrets,omitempty"`
	// Hosts adds /etc/hosts entries (e.g., "10.35.44.41 container4").
	Hosts []string `json:"hosts,omitempty"`
	// Resources specifies CPU and memory limits/reservations.
	Resources Resources `json:"resources,omitempty"`
	// PublishMode controls swarm port publishing: "host" for direct node binding,
	// empty or "ingress" for load-balanced routing mesh (default).
	PublishMode string `json:"publish_mode,omitempty"`
	// User sets the container user (e.g., "1001", "1000:967"). Default is "1001".
	User string `json:"user,omitempty"`
}

// Services is a slice of services.
type Services []Service

// Services gets services from the store.
func (jed *Jed) Services(ctx context.Context) (services Services, err error) {

	services, err = jed.store.Services(ctx)
	return
}

// Find finds a service given its name.
func (services Services) Find(name string) (service Service, err error) {

	for _, service = range services {
		if service.Name == name {
			return
		}
	}
	err = errors.Errorf("service %s not found", name)
	return
}

// CreateService adds a new service definition.
func (jed *Jed) CreateService(ctx context.Context, service Service) (err error) {

	err = service.validate()
	if err != nil {
		return
	}

	services, err := jed.Services(ctx)
	if err != nil {
		return
	}

	_, err = services.Find(service.Name)
	if err == nil {
		err = errors.Errorf("service %s already exists", service.Name)
		return
	}

	err = jed.store.SetService(ctx, service)
	return
}

// DeleteService removes a service definition and its env vars.
func (jed *Jed) DeleteService(ctx context.Context, name string) (err error) {

	services, err := jed.Services(ctx)
	if err != nil {
		return
	}

	_, err = services.Find(name)
	if err != nil {
		return
	}

	err = jed.store.DelService(ctx, name)
	if err != nil {
		err = errors.Wrapf(err, "not deleting %s env", name)
		return
	}

	err = jed.store.DelEnv(ctx, name)
	return
}

// unexported

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

func (service Service) config(env Env) (cfg containerConfig, err error) {

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

func envLines(env map[string]string) (lines []string) {

	// godotenv.Marshal mangled the vals, maybe with quotes? anyway ..

	for key, value := range env {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return
}
