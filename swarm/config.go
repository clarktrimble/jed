package swarm

import (
	"github.com/clarktrimble/giant"
	"github.com/clarktrimble/jed/logger"
	"github.com/pkg/errors"
)

const (
	// DefaultSocket is the conventional Docker socket path.
	DefaultSocket = "/var/run/docker.sock"
)

// Config controls Swarm creation.
type Config struct {
	// Socket is the Docker socket path.
	Socket string `json:"socket" default:"/var/run/docker.sock"`
}

// New creates a Swarm using cfg.Socket.
func (cfg *Config) New(lgr logger.Logger) (*Swarm, error) {
	if cfg == nil {
		return nil, errors.New("swarm config is nil")
	}

	socket := DefaultSocket
	if cfg.Socket != "" {
		socket = cfg.Socket
	}

	clientCfg := &giant.Config{
		BaseUri:    "http://localhost",
		UnixSocket: socket,
	}
	client := clientCfg.NewWithTrippers(lgr)

	return New(client, lgr), nil
}
