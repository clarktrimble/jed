// Package swarm provides a client for deploying to Docker Swarm via socket API.
package swarm

//go:generate moq -out mock_test.go -pkg swarm_test . Client

// Todo: think about streaming events from swarm to buffer for later retrieval

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

// Client sends objects to the Docker socket API.
type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) error
	SendJson(ctx context.Context, method, path string, body io.Reader) ([]byte, error)
}

// Deployer interacts with Docker Swarm.
type Deployer struct {
	client Client
}

// New creates a Deployer.
func New(client Client) *Deployer {
	return &Deployer{client: client}
}

// GetService returns full service info from Docker.
func (d *Deployer) GetService(ctx context.Context, name string) (*ServiceInfo, error) {

	var svc ServiceInfo
	err := d.client.SendObject(ctx, "GET", "/v1.52/services/"+name, nil, &svc)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get service %q", name)
	}

	return &svc, nil
}

// UpdateService updates a service with the given spec.
func (d *Deployer) UpdateService(ctx context.Context, name string, version int, spec any) error {

	path := fmt.Sprintf("/v1.52/services/%s/update?version=%d", name, version)
	err := d.client.SendObject(ctx, "POST", path, spec, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to update service %q", name)
	}

	return nil
}

// CreateService creates a new service.
func (d *Deployer) CreateService(ctx context.Context, spec any) (string, error) {

	var resp idResponse
	err := d.client.SendObject(ctx, "POST", "/v1.52/services/create", spec, &resp)
	if err != nil {
		return "", errors.Wrap(err, "failed to create service")
	}

	return resp.ID, nil
}

// ListSecrets returns all secrets.
func (d *Deployer) ListSecrets(ctx context.Context) ([]Secret, error) {

	var secrets []namedResource
	err := d.client.SendObject(ctx, "GET", "/v1.52/secrets", nil, &secrets)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list secrets")
	}

	result := make([]Secret, len(secrets))
	for i, s := range secrets {
		result[i] = Secret{
			ID:   s.ID,
			Name: s.Spec.Name,
		}
	}

	return result, nil
}

// SecretLatest returns the ID and versioned name of the latest secret by base name.
// Looks for secrets matching {name}_v{N} and returns the highest version.
func (d *Deployer) SecretLatest(ctx context.Context, name string) (id, versionedName string, err error) {

	secrets, err := d.ListSecrets(ctx)
	if err != nil {
		return "", "", err
	}

	return findLatest(secretsToItems(secrets), name, "secret")
}

// CreateSecret creates a versioned secret and returns its ID.
// Creates {name}_v{N} where N is the next version number.
func (d *Deployer) CreateSecret(ctx context.Context, name string, value []byte) (string, error) {

	secrets, err := d.ListSecrets(ctx)
	if err != nil {
		return "", err
	}

	versionedName := fmt.Sprintf("%s_v%d", name, findNextVersion(secretsToItems(secrets), name))
	req := dataCreate{
		Name: versionedName,
		Data: base64.StdEncoding.EncodeToString(value),
	}

	var resp idResponse
	err = d.client.SendObject(ctx, "POST", "/v1.52/secrets/create", req, &resp)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create secret %q", versionedName)
	}

	return resp.ID, nil
}

// ListConfigs returns all configs.
func (d *Deployer) ListConfigs(ctx context.Context) ([]Config, error) {

	var configs []namedResource
	err := d.client.SendObject(ctx, "GET", "/v1.52/configs", nil, &configs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list configs")
	}

	result := make([]Config, len(configs))
	for i, c := range configs {
		result[i] = Config{
			ID:   c.ID,
			Name: c.Spec.Name,
		}
	}

	return result, nil
}

// ConfigLatest returns the ID and versioned name of the latest config by base name.
// Looks for configs matching {name}_v{N} and returns the highest version.
func (d *Deployer) ConfigLatest(ctx context.Context, name string) (id, versionedName string, err error) {

	configs, err := d.ListConfigs(ctx)
	if err != nil {
		return "", "", err
	}

	return findLatest(configsToItems(configs), name, "config")
}

// CreateConfig creates a versioned config and returns its ID.
// Creates {name}_v{N} where N is the next version number.
func (d *Deployer) CreateConfig(ctx context.Context, name string, value []byte) (string, error) {

	configs, err := d.ListConfigs(ctx)
	if err != nil {
		return "", err
	}

	versionedName := fmt.Sprintf("%s_v%d", name, findNextVersion(configsToItems(configs), name))
	req := dataCreate{
		Name: versionedName,
		Data: base64.StdEncoding.EncodeToString(value),
	}

	var resp idResponse
	err = d.client.SendObject(ctx, "POST", "/v1.52/configs/create", req, &resp)
	if err != nil {
		return "", errors.Wrapf(err, "failed to create config %q", versionedName)
	}

	return resp.ID, nil
}

// DeleteConfig deletes a config by ID.
func (d *Deployer) DeleteConfig(ctx context.Context, id string) error {

	err := d.client.SendObject(ctx, "DELETE", "/v1.52/configs/"+id, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to delete config %q", id)
	}

	return nil
}

// CreateNetwork creates an overlay network and returns its ID.
func (d *Deployer) CreateNetwork(ctx context.Context, name string, attachable, encrypted bool) (string, error) {

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

// ListServices returns all services.
func (d *Deployer) ListServices(ctx context.Context) ([]Service, error) {

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
func (d *Deployer) DeleteService(ctx context.Context, name string) error {

	err := d.client.SendObject(ctx, "DELETE", "/v1.52/services/"+name, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to delete service %q", name)
	}

	return nil
}

// ServiceTasks returns tasks for a service.
func (d *Deployer) ServiceTasks(ctx context.Context, serviceName string) ([]Task, error) {

	path := fmt.Sprintf("/v1.52/tasks?filters={\"service\":[\"%s\"]}", serviceName)

	var tasks []taskResponse
	err := d.client.SendObject(ctx, "GET", path, nil, &tasks)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get tasks for %q", serviceName)
	}

	result := make([]Task, len(tasks))
	for i, t := range tasks {
		result[i] = Task{
			ID:        t.ID,
			State:     t.Status.State,
			Error:     t.Status.Err,
			Image:     t.Spec.ContainerSpec.Image,
			Timestamp: t.Status.Timestamp,
		}
	}

	return result, nil
}

// TaskLogs retrieves logs from a swarm task.
func (d *Deployer) TaskLogs(ctx context.Context, taskID, tail string) ([]byte, error) {

	path := fmt.Sprintf("/v1.52/tasks/%s/logs?stdout=true&stderr=true&tail=%s", taskID, tail)

	rawLogs, err := d.client.SendJson(ctx, "GET", path, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get logs for task %q", taskID)
	}

	reader := jed.DecodeLogs(bytes.NewReader(rawLogs))
	logs, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode logs for task %q", taskID)
	}

	return logs, nil
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

type serviceListItem struct {
	ID   string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
	} `json:"Spec"`
}

type taskResponse struct {
	ID     string `json:"ID"`
	Status struct {
		State     string `json:"State"`
		Err       string `json:"Err"`
		Timestamp string `json:"Timestamp"`
	} `json:"Status"`
	Spec struct {
		ContainerSpec struct {
			Image string `json:"Image"`
		} `json:"ContainerSpec"`
	} `json:"Spec"`
}

type idResponse struct {
	ID string `json:"ID"`
}

type namedResource struct {
	ID   string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
	} `json:"Spec"`
}

type dataCreate struct {
	Name string `json:"Name"`
	Data string `json:"Data"`
}

type networkCreate struct {
	Name       string            `json:"Name"`
	Driver     string            `json:"Driver"`
	Attachable bool              `json:"Attachable"`
	Options    map[string]string `json:"Options,omitempty"`
}

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
