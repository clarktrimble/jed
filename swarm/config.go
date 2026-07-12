package swarm

import (
	"github.com/clarktrimble/giant"
	"github.com/clarktrimble/jed/logger"
)

// Config controls Swarm creation. Fields are populated by launch (envconfig)
// from their default tags.
type Config struct {
	// Socket is the Docker socket path.
	Socket string `json:"socket" default:"/var/run/docker.sock"`
}

// New creates a Swarm from Config.
func (cfg *Config) New(lgr logger.Logger) *Swarm {
	clientCfg := &giant.Config{
		BaseUri:    "http://localhost",
		UnixSocket: cfg.Socket,
	}
	client := clientCfg.NewWithTrippers(lgr)

	return New(client, lgr)
}
