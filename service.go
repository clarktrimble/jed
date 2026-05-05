package jed

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

const (
	// DefaultCPULimit is the default CPU limit applied by runtimes.
	DefaultCPULimit = "0.5"
	// DefaultMemLimit is the default memory limit applied by runtimes.
	DefaultMemLimit = "128M"
	// DefaultCPUReserve is the default CPU reservation applied by runtimes.
	DefaultCPUReserve = "0.1"
	// DefaultMemReserve is the default memory reservation applied by runtimes.
	DefaultMemReserve = "64M"
)

const (
	// RestartNone disables runtime restarts.
	RestartNone = "none"
	// RestartOnFailure restarts tasks or containers that fail.
	RestartOnFailure = "on-failure"
	// RestartAny restarts tasks or containers after any exit.
	RestartAny = "any"
)

// RestartPolicy specifies whether a runtime should restart failed tasks or containers.
//
// Condition is one of "none", "on-failure", or "any". Empty means "none".
// MaxAttempts limits restart attempts where supported. Nil means runtime default.
type RestartPolicy struct {
	Condition   string `json:"condition,omitempty"`
	MaxAttempts *int   `json:"max_attempts,omitempty"`
}

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
// URLs containing {{template}} vars are accepted without parsing.
func (l Link) Validate() error {
	if strings.Contains(l.Url, "{{") { // Todo: dehax
		return nil
	}
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
	Name string `json:"name"`
	// Image is the Docker image (e.g., "postgres:16").
	Image string `json:"image"`
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
	// Restart specifies restart behavior. Empty means no restart.
	Restart RestartPolicy `json:"restart,omitempty"`
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
}

// Services is a slice of services.
type Services []Service

// Find finds a service by name.
func (services Services) Find(name string) (service Service, err error) {
	for _, service = range services {
		if service.Name == name {
			return
		}
	}
	err = errors.Errorf("service %s not found", name)
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
	if service.Restart.Condition != "" && service.Restart.Condition != RestartNone && service.Restart.Condition != RestartOnFailure && service.Restart.Condition != RestartAny {
		issues = append(issues, fmt.Sprintf("restart condition %q must be one of %q, %q, or %q", service.Restart.Condition, RestartNone, RestartOnFailure, RestartAny))
	}
	if service.Restart.MaxAttempts != nil && *service.Restart.MaxAttempts < 0 {
		issues = append(issues, "restart max_attempts cannot be negative")
	}
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

func validUser(user string) bool {
	parts := strings.SplitN(user, ":", 2)
	for _, p := range parts {
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}
