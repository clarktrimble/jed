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
	Spec          ServiceSpec     `json:"Spec"`
	PreviousSpec  json.RawMessage `json:"PreviousSpec,omitempty"`
	Endpoint      ServiceEndpoint `json:"Endpoint"`
	UpdateStatus  UpdateStatus    `json:"UpdateStatus"`
	ServiceStatus ServiceStatus   `json:"ServiceStatus"`
	CreatedAt     time.Time       `json:"CreatedAt"`
	UpdatedAt     time.Time       `json:"UpdatedAt"`
}

// ServiceSpec is Docker's user-modifiable swarm service configuration as read
// back from Docker (inbound). It is deliberately kept separate from the typed
// outbound Spec in spec.go: substructures Jed does not inspect stay as
// json.RawMessage so Restart can round-trip the current spec verbatim (only
// bumping ForceUpdate) without modeling Docker's full schema.
type ServiceSpec struct {
	Name           string            `json:"Name,omitempty"`
	Labels         map[string]string `json:"Labels,omitempty"`
	TaskTemplate   TaskSpec          `json:"TaskTemplate"`
	Mode           json.RawMessage   `json:"Mode,omitempty"`
	UpdateConfig   json.RawMessage   `json:"UpdateConfig,omitempty"`
	RollbackConfig json.RawMessage   `json:"RollbackConfig,omitempty"`
	Networks       json.RawMessage   `json:"Networks,omitempty"`
	EndpointSpec   json.RawMessage   `json:"EndpointSpec,omitempty"`
}

// TaskSpec is Docker's user-modifiable swarm task configuration.
type TaskSpec struct {
	PluginSpec            json.RawMessage `json:"PluginSpec,omitempty"`
	ContainerSpec         json.RawMessage `json:"ContainerSpec,omitempty"`
	NetworkAttachmentSpec json.RawMessage `json:"NetworkAttachmentSpec,omitempty"`
	Resources             json.RawMessage `json:"Resources,omitempty"`
	RestartPolicy         json.RawMessage `json:"RestartPolicy,omitempty"`
	Placement             json.RawMessage `json:"Placement,omitempty"`
	Networks              json.RawMessage `json:"Networks,omitempty"`
	LogDriver             json.RawMessage `json:"LogDriver,omitempty"`
	ForceUpdate           uint64          `json:"ForceUpdate,omitempty"`
	Runtime               string          `json:"Runtime,omitempty"`
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

	var resp IDResponse
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
