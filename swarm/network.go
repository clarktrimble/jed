package swarm

import (
	"context"
	"strings"

	"github.com/pkg/errors"
)

// ErrNetworkExists is returned when a swarm network already exists.
var ErrNetworkExists = errors.New("swarm network already exists")

// IsNetworkExists reports whether err indicates an existing swarm network.
func IsNetworkExists(err error) bool {
	return errors.Is(err, ErrNetworkExists)
}

// CreateNetwork creates an overlay network and returns its ID.
func (d *Swarm) CreateNetwork(ctx context.Context, name string, attachable, encrypted bool) (string, error) {

	req := networkCreate{
		Name:       name,
		Driver:     "overlay",
		Attachable: attachable,
	}
	if encrypted {
		req.Options = map[string]string{"encrypted": "true"}
	}

	var resp IDResponse
	err := d.client.SendObject(ctx, "POST", "/v1.52/networks/create", req, &resp)
	if err != nil {
		if isNetworkExistsError(err, name) {
			return "", errors.Wrapf(ErrNetworkExists, "failed to create network %q: %v", name, err)
		}
		return "", errors.Wrapf(err, "failed to create network %q", name)
	}

	return resp.ID, nil
}

// EnsureNetwork creates an overlay network unless it already exists.
func (d *Swarm) EnsureNetwork(ctx context.Context, name string, attachable, encrypted bool) error {
	_, err := d.CreateNetwork(ctx, name, attachable, encrypted)
	if IsNetworkExists(err) {
		return nil
	}
	return err
}

func isNetworkExistsError(err error, name string) bool {
	if err == nil {
		return false
	}

	msg := err.Error()
	return strings.Contains(msg, "status code 409") &&
		strings.Contains(msg, "network with name "+name+" already exists")
}

type networkCreate struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Attachable bool              `json:"Attachable"`
	Options    map[string]string `json:"Options,omitempty"`
}
