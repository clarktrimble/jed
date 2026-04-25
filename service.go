package jed

import (
	"context"
	"fmt"
	"maps"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Resource defaults
const (
	DefaultCPULimit   = "0.5"
	DefaultMemLimit   = "128M"
	DefaultCPUReserve = "0.1"
	DefaultMemReserve = "64M"
)

// Traefik specifies traefik routing configuration.
type Traefik struct {
	Port            string `json:"port"`
	PathPrefixStrip bool   `json:"path_prefix_strip,omitempty"`
}

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

// Link is a labeled URL.
type Link struct {
	Text string `json:"text"`
	Url  string `json:"url"`
}

// Validate checks that the URL is parseable and has a scheme.
func (l Link) Validate() error {
	u, err := url.Parse(l.Url)
	if err != nil {
		return errors.Errorf("invalid url %q: %v", l.Url, err)
	}
	if u.Scheme == "" {
		return errors.Errorf("url %q missing scheme", l.Url)
	}
	return nil
}

// Note is a timestamped, authored comment.
type Note struct {
	Ts      time.Time `json:"ts"`
	Author  string    `json:"author"`
	Content string    `json:"content"`
}

// About holds user-facing descriptive information for a service.
type About struct {
	Desc  string `json:"desc,omitempty"`
	Links []Link `json:"links,omitempty"`
	Notes []Note `json:"notes,omitempty"`
}

// Service is a service's configuration.
type Service struct {
	// Name is the service name (e.g., "postgres").
	// Todo: add json tag, requires store migration for existing data
	Name string
	// Image is the Docker image (e.g., "postgres:16").
	// Todo: add json tag, requires store migration for existing data
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
	// Configs maps swarm config base names to target paths (e.g., "myapp_config": "/etc/myapp/app.conf").
	Configs map[string]string `json:"configs,omitempty"`
	// Hosts adds /etc/hosts entries (e.g., "10.35.44.41 container4").
	Hosts []string `json:"hosts,omitempty"`
	// Resources specifies CPU and memory limits/reservations.
	Resources Resources `json:"resources"`
	// PublishMode controls swarm port publishing: "host" for direct node binding,
	// empty or "ingress" for load-balanced routing mesh (default).
	PublishMode string `json:"publish_mode,omitempty"`
	// User sets the container user (e.g., "1001", "1000:967"). Default is "1001".
	User string `json:"user,omitempty"`
	// About holds user-facing descriptive information.
	About About `json:"about"`
	// Traefik enables traefik routing label generation.
	Traefik *Traefik `json:"traefik,omitempty"`
	// Replicas is the number of service instances to run.
	Replicas int `json:"replicas,omitempty"`
	// RestartService enables swarm restart on failure (default false: no restart).
	RestartService bool `json:"restart_service,omitempty"`
	// RestartAttempts is max restart attempts before giving up (default 1, 0 for unlimited).
	RestartAttempts *int `json:"restart_attempts,omitempty"`
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

	err = service.Validate()
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

// Validate checks that the service has valid configuration.
func (service Service) Validate() error {
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
	if service.User != "" && !validUser(service.User) {
		issues = append(issues, fmt.Sprintf("user %q must be uid or uid:gid with numeric values", service.User))
	}
	// Todo: sort out Restart (container) vs RestartService (swarm) validation
	if service.Replicas < 0 {
		issues = append(issues, "replicas cannot be negative")
	}
	for _, link := range service.About.Links {
		err := link.Validate()
		if err != nil {
			issues = append(issues, err.Error())
		}
	}

	if len(issues) > 0 {
		return errors.Errorf("service %s invalid: %s", service.Name, strings.Join(issues, ", "))
	}

	return nil
}

// unexported

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

func validUser(user string) bool {
	parts := strings.SplitN(user, ":", 2)
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

func envLines(env map[string]string) (lines []string) {

	// godotenv.Marshal mangled the vals, maybe with quotes? anyway ..

	for key, value := range env {
		lines = append(lines, fmt.Sprintf("%s=%s", key, value))
	}

	return
}
