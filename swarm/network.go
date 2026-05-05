package swarm

import (
	"context"

	"github.com/pkg/errors"
)

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

	var resp idResponse
	err := d.client.SendObject(ctx, "POST", "/v1.52/networks/create", req, &resp)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create network %q", name)
	}

	return resp.ID, nil
}

type networkCreate struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Attachable bool              `json:"Attachable"`
	Options    map[string]string `json:"Options,omitempty"`
}
