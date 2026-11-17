// Package swarm deploys services to Docker Swarm via socket API.
//
// Features:
//
//   - Create, update, and delete services
//   - Versioned secrets and configs (e.g., "db_pass_v3")
//   - Host or ingress port publishing
//   - Bind mounts, named volumes, and NFSv3 network volumes
//   - Configurable UID:GID and supplementary groups
//   - Resource limits (CPU as decimal, memory with M suffix)
//   - Extra hosts (/etc/hosts entries)
//   - Custom command override
//   - Service labels
//   - Task inspection and log retrieval
//
// Simplifying Constraints:
//
//   - Read-only root filesystem
//   - Secrets mounted with the service UID/GID, mode 0400
//   - Runtime-wide automatic restart settings; on-failure retains one retry
//   - Update order stop-first, pause on failure
//   - JSON file logging with 10MB rotation, 3 files
package swarm

//go:generate moq -out mock_test.go -pkg swarm_test . Client

// Todo: think about streaming events from swarm to buffer for later retrieval

import (
	"context"
	"io"
	"time"

	"github.com/clarktrimble/jed/logger"
	"github.com/pkg/errors"
)

// Client talks to the Docker API.
type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) error
	SendJson(ctx context.Context, method, path string, body io.Reader) ([]byte, error)
	StreamLines(ctx context.Context, path string) (<-chan []byte, error)
}

// Swarm interacts with Docker Swarm.
type Swarm struct {
	client             Client
	logger             logger.Logger
	restartDelay       time.Duration
	restartMaxAttempts int
	restartWindow      time.Duration
}

// ErrServiceNotFound is returned when a swarm service does not exist.
var ErrServiceNotFound = errors.New("swarm service not found")

// New creates a Swarm with default runtime settings and the supplied client.
// Use Config.New to customize runtime settings.
func New(client Client, logger logger.Logger) *Swarm {
	return &Swarm{
		client:             client,
		logger:             logger,
		restartDelay:       5 * time.Second,
		restartMaxAttempts: 5,
		restartWindow:      2 * time.Minute,
	}
}

// IDResponse identifies a Docker resource created by an API call.
type IDResponse struct {
	// ID is the Docker resource ID.
	ID string `json:"ID"`
}
