// Package swarm provides a client for deploying to Docker Swarm via socket API.
package swarm

//go:generate moq -out mock_test.go -pkg swarm_test . Client

import (
	"context"
	"encoding/base64"
	"fmt"
	"strconv"
	"strings"

	"github.com/pkg/errors"
)

// Client sends objects to the Docker socket API.
type Client interface {
	SendObject(ctx context.Context, method, path string, snd, rcv any) error
}

// Deployer interacts with Docker Swarm.
type Deployer struct {
	client Client
}

// New creates a Deployer.
func New(client Client) *Deployer {
	return &Deployer{client: client}
}

// ServiceVersion returns the current version index for a service.
func (d *Deployer) ServiceVersion(ctx context.Context, name string) (int, error) {

	var svc serviceResponse
	err := d.client.SendObject(ctx, "GET", "/v1.52/services/"+name, nil, &svc)
	if err != nil {
		return 0, errors.Wrapf(err, "failed to get service %q", name)
	}

	return svc.Version.Index, nil
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

	var bestID, bestName string
	var bestVersion int

	prefix := name + "_v"
	for _, s := range secrets {
		if strings.HasPrefix(s.Name, prefix) {
			vStr := strings.TrimPrefix(s.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v > bestVersion {
				bestVersion = v
				bestID = s.ID
				bestName = s.Name
			}
		}
	}

	if bestID == "" {
		return "", "", errors.Errorf("secret %q not found (no %s_v* versions)", name, name)
	}

	return bestID, bestName, nil
}

// CreateSecret creates a versioned secret and returns its ID.
// Creates {name}_v{N} where N is the next version number.
func (d *Deployer) CreateSecret(ctx context.Context, name string, value []byte) (string, error) {

	// Find next version
	secrets, err := d.ListSecrets(ctx)
	if err != nil {
		return "", err
	}

	nextVersion := 1
	prefix := name + "_v"
	for _, s := range secrets {
		if strings.HasPrefix(s.Name, prefix) {
			vStr := strings.TrimPrefix(s.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v >= nextVersion {
				nextVersion = v + 1
			}
		}
	}

	versionedName := fmt.Sprintf("%s_v%d", name, nextVersion)
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

	var bestID, bestName string
	var bestVersion int

	prefix := name + "_v"
	for _, c := range configs {
		if strings.HasPrefix(c.Name, prefix) {
			vStr := strings.TrimPrefix(c.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v > bestVersion {
				bestVersion = v
				bestID = c.ID
				bestName = c.Name
			}
		}
	}

	if bestID == "" {
		return "", "", errors.Errorf("config %q not found (no %s_v* versions)", name, name)
	}

	return bestID, bestName, nil
}

// CreateConfig creates a versioned config and returns its ID.
// Creates {name}_v{N} where N is the next version number.
func (d *Deployer) CreateConfig(ctx context.Context, name string, value []byte) (string, error) {

	// Find next version
	configs, err := d.ListConfigs(ctx)
	if err != nil {
		return "", err
	}

	nextVersion := 1
	prefix := name + "_v"
	for _, c := range configs {
		if strings.HasPrefix(c.Name, prefix) {
			vStr := strings.TrimPrefix(c.Name, prefix)
			v, err := strconv.Atoi(vStr)
			if err == nil && v >= nextVersion {
				nextVersion = v + 1
			}
		}
	}

	versionedName := fmt.Sprintf("%s_v%d", name, nextVersion)
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

// unexported

type serviceResponse struct {
	Version struct {
		Index int `json:"Index"`
	} `json:"Version"`
}

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
