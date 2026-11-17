package swarm

import (
	"time"

	"github.com/clarktrimble/giant"
	"github.com/clarktrimble/jed/logger"
)

// Config controls Swarm creation. Fields are populated by launch (envconfig)
// from their default tags.
type Config struct {
	// Socket is the Docker socket path.
	Socket string `json:"socket" default:"/var/run/docker.sock"`
	// RestartDelay is the delay before an automatic task restart.
	RestartDelay time.Duration `json:"restart_delay" default:"5s" split_words:"true"`
	// RestartMaxAttempts limits retries for "any"; zero means unlimited.
	RestartMaxAttempts int `json:"restart_max_attempts" default:"5" split_words:"true"`
	// RestartWindow is the retry-count lookback period for "any"; zero counts all retries.
	RestartWindow time.Duration `json:"restart_window" default:"2m" split_words:"true"`
}

// New creates a Swarm from Config.
func (cfg *Config) New(lgr logger.Logger) *Swarm {
	clientCfg := &giant.Config{
		BaseUri:    "http://localhost",
		UnixSocket: cfg.Socket,
	}
	client := clientCfg.NewWithTrippers(lgr)

	sw := New(client, lgr)
	sw.restartDelay = cfg.RestartDelay
	sw.restartMaxAttempts = cfg.RestartMaxAttempts
	sw.restartWindow = cfg.RestartWindow
	return sw
}
