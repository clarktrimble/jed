package swarm

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/pkg/errors"
)

// Service represents a swarm service.
type Service struct {
	ID   string
	Name string
}

// ServiceInfo holds the full response from the Docker service endpoint.
type ServiceInfo struct {
	ID      string `json:"ID"`
	Version struct {
		Index int `json:"Index"`
	} `json:"Version"`
	Spec          json.RawMessage `json:"Spec"`
	PreviousSpec  json.RawMessage `json:"PreviousSpec,omitempty"`
	Endpoint      ServiceEndpoint `json:"Endpoint"`
	UpdateStatus  UpdateStatus    `json:"UpdateStatus"`
	ServiceStatus ServiceStatus   `json:"ServiceStatus"`
	CreatedAt     time.Time       `json:"CreatedAt"`
	UpdatedAt     time.Time       `json:"UpdatedAt"`
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

// GetService returns full service info from Docker.
func (d *Swarm) GetService(ctx context.Context, name string) (*ServiceInfo, error) {

	path := fmt.Sprintf("/v1.52/services?status=true&filters={\"name\":[\"%s\"]}", name)

	var svcs []ServiceInfo
	err := d.client.SendObject(ctx, "GET", path, nil, &svcs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get service %q", name)
	}

	if len(svcs) == 0 {
		return nil, errors.Wrapf(ErrServiceNotFound, "service %q", name)
	}

	return &svcs[0], nil
}

// ListServices returns all services.
func (d *Swarm) ListServices(ctx context.Context) ([]Service, error) {

	var svcs []serviceListItem
	err := d.client.SendObject(ctx, "GET", "/v1.52/services", nil, &svcs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list services")
	}

	result := make([]Service, len(svcs))
	for i, s := range svcs {
		result[i] = Service{
			ID:   s.ID,
			Name: s.Spec.Name,
		}
	}

	return result, nil
}

// DeleteService deletes a service by name.
func (d *Swarm) DeleteService(ctx context.Context, name string) error {

	err := d.client.SendObject(ctx, "DELETE", "/v1.52/services/"+name, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to delete service %q", name)
	}

	return nil
}

func (d *Swarm) updateService(ctx context.Context, name string, version int, spec any) error {

	path := fmt.Sprintf("/v1.52/services/%s/update?version=%d", name, version)
	err := d.client.SendObject(ctx, "POST", path, spec, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to update service %q", name)
	}

	return nil
}

func (d *Swarm) createService(ctx context.Context, spec any) (string, error) {

	var resp idResponse
	err := d.client.SendObject(ctx, "POST", "/v1.52/services/create", spec, &resp)
	if err != nil {
		return "", errors.Wrap(err, "failed to create service")
	}

	return resp.ID, nil
}

type serviceListItem struct {
	ID   string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
	} `json:"Spec"`
	ServiceStatus ServiceStatus `json:"ServiceStatus"`
	UpdateStatus  UpdateStatus  `json:"UpdateStatus"`
}
