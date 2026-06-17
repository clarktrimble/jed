package swarm

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
)

// GetService returns full service info from Docker.
func (d *Swarm) GetService(ctx context.Context, name string) (*ServiceInfo, error) {

	path := fmt.Sprintf("/v1.52/services?status=true&filters={\"name\":[\"%s\"]}", name)

	var svcs []ServiceInfo
	err := d.client.SendObject(ctx, "GET", path, nil, &svcs)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get service %q", name)
	}

	if len(svcs) == 0 {
		return nil, errors.Errorf("service %q not found", name)
	}

	return &svcs[0], nil
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

type idResponse struct {
	ID string `json:"ID"`
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

type serviceListItem struct {
	ID   string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
	} `json:"Spec"`
	ServiceStatus ServiceStatus `json:"ServiceStatus"`
	UpdateStatus  UpdateStatus  `json:"UpdateStatus"`
}

// Statuses returns the status of all services.
func (d *Swarm) Statuses(ctx context.Context) (map[string]Status, error) {

	var svcs []serviceListItem
	err := d.client.SendObject(ctx, "GET", "/v1.52/services?status=true", nil, &svcs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list services")
	}

	result := make(map[string]Status, len(svcs))
	for _, s := range svcs {
		result[s.Spec.Name] = computeStatus(s.ServiceStatus, s.UpdateStatus)
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

// ListSecrets returns all secrets.
func (d *Swarm) ListSecrets(ctx context.Context) ([]Secret, error) {

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

type namedResource struct {
	ID   string `json:"ID"`
	Spec struct {
		Name string `json:"Name"`
	} `json:"Spec"`
}

// CreateSecret creates a versioned secret and returns its ID.
// Creates {name}_v{N} where N is the next version number.
func (d *Swarm) CreateSecret(ctx context.Context, name string, value []byte) (string, error) {

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

type dataCreate struct {
	Name string `json:"Name"`
	Data string `json:"Data"`
}

// DeleteSecret deletes a secret by ID.
func (d *Swarm) DeleteSecret(ctx context.Context, id string) error {

	err := d.client.SendObject(ctx, "DELETE", "/v1.52/secrets/"+id, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to delete secret %q", id)
	}

	return nil
}

// ListConfigs returns all configs.
func (d *Swarm) ListConfigs(ctx context.Context) ([]Config, error) {

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

// CreateConfig creates a versioned config and returns its ID.
// Creates {name}_v{N} where N is the next version number.
func (d *Swarm) CreateConfig(ctx context.Context, name string, value []byte) (string, error) {

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
func (d *Swarm) DeleteConfig(ctx context.Context, id string) error {

	err := d.client.SendObject(ctx, "DELETE", "/v1.52/configs/"+id, nil, nil)
	if err != nil {
		return errors.Wrapf(err, "failed to delete config %q", id)
	}

	return nil
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

// ServiceTasks returns tasks for a service.
func (d *Swarm) ServiceTasks(ctx context.Context, serviceName string) ([]Task, error) {

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

// TaskLogs retrieves logs from a swarm task.
func (d *Swarm) TaskLogs(ctx context.Context, taskID, tail string) ([]byte, error) {

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

// Status returns the current status of a deployed service.
//
// Returns status:
//   - StatusStopped - DesiredTasks == 0 and no tasks running
//   - StatusPending - transitioning: deploying or stopping
//   - StatusError   - deploy failed (paused) or task count mismatch
//   - StatusRunning - healthy, all desired tasks running
//
// Note: Job mode services (replicated-job, global-job) would need different logic.
// Todo: consider rollback, how hard will this be to support from a ux sanity perspective?
func (d *Swarm) Status(ctx context.Context, name string) (Status, error) {

	svc, err := d.GetService(ctx, name)
	if err != nil {
		return "", err
	}

	return computeStatus(svc.ServiceStatus, svc.UpdateStatus), nil
}

// computeStatus determines the status from ServiceStatus and UpdateStatus.
func computeStatus(ss ServiceStatus, us UpdateStatus) Status {
	switch {
	case ss.DesiredTasks == 0 && ss.RunningTasks == 0:
		return StatusStopped
	case ss.DesiredTasks == 0 && ss.RunningTasks > 0:
		return StatusPending // stopping
	case us.State == "updating":
		return StatusPending // deploying
	case us.State == "paused":
		return StatusError // deploy failed
	case ss.RunningTasks == ss.DesiredTasks:
		return StatusRunning
	default:
		return StatusError
	}
}
