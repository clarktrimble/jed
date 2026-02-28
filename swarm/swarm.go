// Package swarm deploys services to Docker Swarm via socket API.
//
// Features:
//
//   - Create, update, and delete services
//   - Versioned secrets and configs (e.g., "db_pass_v3")
//   - Host or ingress port publishing
//   - Bind mounts and named volumes
//   - Configurable UID:GID (defaults to 1001)
//   - Resource limits (CPU as decimal, memory with M suffix)
//   - Extra hosts (/etc/hosts entries)
//   - Custom command override
//   - Service labels
//   - Task inspection and log retrieval
//
// Simplifying Constraints:
//
//   - Single replica only
//   - Read-only root filesystem
//   - Secrets mounted with UID/GID 1001, mode 0444
//   - Restart on-failure with 5s delay, max 3 attempts
//   - Update order stop-first, rollback on failure
//   - JSON file logging with 10MB rotation, 3 files
package swarm

//go:generate moq -out mock_test.go -pkg swarm_test . Client

// Todo: think about streaming events from swarm to buffer for later retrieval

import (
	"context"
	"encoding/json"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/pkg/errors"
)

// Client talks to the Docker API.
type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) error
	SendJson(ctx context.Context, method, path string, body io.Reader) ([]byte, error)
}

// Swarm interacts with Docker Swarm.
type Swarm struct {
	client Client
}

// New creates a Swarm.
func New(client Client) *Swarm {
	return &Swarm{client: client}
}

// SecretLatest returns the ID and versioned name of the latest secret by base name.
// Looks for secrets matching {name}_v{N} and returns the highest version.
func (d *Swarm) SecretLatest(ctx context.Context, name string) (id, versionedName string, err error) {

	secrets, err := d.ListSecrets(ctx)
	if err != nil {
		return "", "", err
	}

	return findLatest(secretsToItems(secrets), name, "secret")
}

// ConfigLatest returns the ID and versioned name of the latest config by base name.
// Looks for configs matching {name}_v{N} and returns the highest version.
// Todo: add test data with configs and test this
func (d *Swarm) ConfigLatest(ctx context.Context, name string) (id, versionedName string, err error) {

	configs, err := d.ListConfigs(ctx)
	if err != nil {
		return "", "", err
	}

	return findLatest(configsToItems(configs), name, "config")
}

// Service represents a swarm service.
type Service struct {
	ID   string
	Name string
}

// Task represents a service task.
type Task struct {
	ID        string
	State     string
	Error     string
	Image     string
	Timestamp string
}

// Secret represents a swarm secret.
type Secret struct {
	ID   string
	Name string
}

// Config represents a swarm config.
type Config struct {
	ID   string
	Name string
}

// ServiceInfo holds the full response from the Docker service endpoint.
type ServiceInfo struct {
	Version struct {
		Index int `json:"Index"`
	} `json:"Version"`
	Spec         json.RawMessage `json:"Spec"`
	PreviousSpec json.RawMessage `json:"PreviousSpec,omitempty"`
	Endpoint     ServiceEndpoint `json:"Endpoint"`
	UpdateStatus UpdateStatus    `json:"UpdateStatus"`
	CreatedAt    time.Time       `json:"CreatedAt"`
	UpdatedAt    time.Time       `json:"UpdatedAt"`
}

// ServiceEndpoint represents a service's network endpoint.
type ServiceEndpoint struct {
	Ports      []PortConfig `json:"Ports"`
	VirtualIPs []VirtualIP  `json:"VirtualIPs"`
}

// PortConfig represents a published port.
type PortConfig struct {
	Protocol      string `json:"Protocol"`
	TargetPort    int    `json:"TargetPort"`
	PublishedPort int    `json:"PublishedPort"`
	PublishMode   string `json:"PublishMode"`
}

// VirtualIP represents a service's virtual IP on a network.
type VirtualIP struct {
	NetworkID string `json:"NetworkID"`
	Addr      string `json:"Addr"`
}

// UpdateStatus represents the status of a service update.
type UpdateStatus struct {
	State       string    `json:"State"`
	Message     string    `json:"Message"`
	StartedAt   time.Time `json:"StartedAt"`
	CompletedAt time.Time `json:"CompletedAt"`
}

// unexported

// namedItem is used by version helpers.
type namedItem struct {
	ID   string
	Name string
}

func secretsToItems(secrets []Secret) []namedItem {
	items := make([]namedItem, len(secrets))
	for i, s := range secrets {
		items[i] = namedItem(s)
	}
	return items
}

func configsToItems(configs []Config) []namedItem {
	items := make([]namedItem, len(configs))
	for i, c := range configs {
		items[i] = namedItem(c)
	}
	return items
}

func findLatest(items []namedItem, baseName, resourceType string) (id, name string, err error) {
	var bestID, bestName string
	var bestVersion int

	prefix := baseName + "_v"
	for _, item := range items {
		if strings.HasPrefix(item.Name, prefix) {
			vStr := strings.TrimPrefix(item.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v > bestVersion {
				bestVersion = v
				bestID = item.ID
				bestName = item.Name
			}
		}
	}

	if bestID == "" {
		return "", "", errors.Errorf("%s %q not found (no %s_v* versions)", resourceType, baseName, baseName)
	}

	return bestID, bestName, nil
}

func findNextVersion(items []namedItem, baseName string) int {
	nextVersion := 1
	prefix := baseName + "_v"

	for _, item := range items {
		if strings.HasPrefix(item.Name, prefix) {
			vStr := strings.TrimPrefix(item.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v >= nextVersion {
				nextVersion = v + 1
			}
		}
	}

	return nextVersion
}
