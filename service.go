package jed

import (
	"fmt"
	"net/url"
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

// Traefik specifies traefik routing configuration.
type Traefik struct {
	// Port is the container port that traefik should route traffic to.
	Port string `json:"port"`
	// PathPrefixStrip enables stripping the matched path prefix before forwarding.
	PathPrefixStrip bool `json:"path_prefix_strip,omitempty"`
}

// Resources specifies CPU and memory limits and reservations.
// CPU values are decimal strings (e.g., "0.5" for half a CPU).
// Memory values require M suffix (e.g., "128M" for 128 megabytes).
type Resources struct {
	// CPULimit is the maximum CPU allocation, expressed as decimal CPUs.
	CPULimit string `json:"cpu_limit,omitempty"`
	// MemLimit is the maximum memory allocation, expressed with an M suffix.
	MemLimit string `json:"mem_limit,omitempty"`
	// CPUReserve is the reserved CPU allocation, expressed as decimal CPUs.
	CPUReserve string `json:"cpu_reserve,omitempty"`
	// MemReserve is the reserved memory allocation, expressed with an M suffix.
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
	// Text is the human-readable link label.
	Text string `json:"text"`
	// Url is the target URL. It may contain {{template}} variables rendered from Env.
	Url string `json:"url"`
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
	// Ts is when the note was written.
	Ts time.Time `json:"ts"`
	// Author identifies who wrote the note.
	Author string `json:"author"`
	// Content is the note body.
	Content string `json:"content"`
}

// About holds user-facing descriptive information for a service.
type About struct {
	// Desc is a short human-readable description of the service.
	Desc string `json:"desc,omitempty"`
	// Links are related URLs for operators or users.
	Links []Link `json:"links,omitempty"`
	// Notes are timestamped operational notes about the service.
	Notes []Note `json:"notes,omitempty"`
}

// Todo: work on "Service"
//   Ports and Hosts are vestigial?
//   Network is always the same
//   Restart is weird
//   Etc.

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
	// LocalVolumes lists container paths backed by convention-derived local bind mounts.
	LocalVolumes []string `json:"local_volumes,omitempty"`
	// Network is the Docker network name.
	Network string `json:"network"`
	// Restart specifies restart behavior. Empty means no restart.
	Restart string `json:"restart,omitempty"`
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
	// User sets the container user (e.g., "1000", "1000:967"). Default is "1000".
	User string `json:"user,omitempty"`
	// Groups lists supplementary group GIDs for the container (e.g., "967").
	Groups []string `json:"groups,omitempty"`
	// About holds user-facing descriptive information.
	About About `json:"about"`
	// Traefik enables traefik routing label generation.
	Traefik *Traefik `json:"traefik,omitempty"`
}

// Services is a slice of services.
// Todo: check that we use this where applicable.
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

// ByName groups services by logical service name.
func (services Services) ByName() map[string]Services {
	byName := make(map[string]Services)
	for _, service := range services {
		byName[service.Name] = append(byName[service.Name], service)
	}
	return byName
}

// Validate checks that the service has valid configuration.
// Todo: consider pre/post substitution validation.
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
		issues = append(issues, fmt.Sprintf("user %q must be uid or uid:gid with numeric values or {{VAR}} templates", service.User))
	}
	for _, group := range service.Groups {
		if !validUserPart(group) {
			issues = append(issues, fmt.Sprintf("group %q must be a numeric gid or {{VAR}} template", group))
		}
	}
	for _, volume := range service.LocalVolumes {
		if strings.HasPrefix(volume, "{{") { // allow full-path templates; rendered values are validated in Spec
			continue
		}
		err := validateLocalVolume(volume)
		if err != nil {
			issues = append(issues, err.Error())
		}
	}
	if service.Restart != "" && service.Restart != RestartNone && service.Restart != RestartOnFailure && service.Restart != RestartAny {
		issues = append(issues, fmt.Sprintf("restart %q must be one of %q, %q, or %q", service.Restart, RestartNone, RestartOnFailure, RestartAny))
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

func validateLocalVolume(volume string) error {
	_, err := cleanLocalPath("local volume", volume)
	return err
}

func validUser(user string) bool {
	parts := strings.Split(user, ":")
	if len(parts) > 2 {
		return false
	}
	for _, p := range parts {
		if !validUserPart(p) {
			return false
		}
	}
	return true
}

func validUserPart(part string) bool {
	if part == "" {
		return false
	}
	if validNumericID(part) {
		return true
	}
	return validTemplateVar(part)
}

func validNumericID(s string) bool {
	for _, r := range s {
		if r < '0' || r > '9' {
			return false
		}
	}
	return true
}

func validTemplateVar(s string) bool {
	if !strings.HasPrefix(s, "{{") || !strings.HasSuffix(s, "}}") {
		return false
	}
	name := strings.TrimSuffix(strings.TrimPrefix(s, "{{"), "}}")
	if name == "" {
		return false
	}
	for i, r := range name {
		switch {
		case r >= 'A' && r <= 'Z':
		case r >= 'a' && r <= 'z':
		case r == '_':
		case i > 0 && r >= '0' && r <= '9':
		default:
			return false
		}
	}
	return true
}
