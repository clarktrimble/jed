package swarm

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"time"

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
// Todo: Further work needed to distinguish error causes:
//   - UpdateStatus.State values: "updating", "paused", "completed",
//     "rollback_started", "rollback_paused", "rollback_completed"
//   - UpdateStatus.Message may contain error details
//   - Our UpdateConfig uses FailureAction: "rollback", so failed deploys trigger rollback
//   - Need to capture actual rollback scenarios to understand what Docker returns
//   - Consider whether "pending" state is needed for startup vs actual errors
//   - Tasks endpoint has detailed error info but selecting the right task is tricky
//   - dont forget about events from docker and state in jed.db which could be helpful
//   - in any case, rollback is the one we want to nail here? (FailureAction: pause for now)
//
// Note: Job mode services (replicated-job, global-job) would need different logic:
//   - CompletedTasks is only populated for job modes (always 0 for replicated/global)
//   - Job success: CompletedTasks == DesiredTasks
//   - Current logic wrongly reports "error" for completed jobs
func (d *Swarm) Status(ctx context.Context, name string) (string, error) {

	svc, err := d.GetService(ctx, name)
	if err != nil {
		return "", err
	}

	ss := svc.ServiceStatus

	switch {
	case ss.DesiredTasks == 0:
		return "stopped", nil
	case ss.RunningTasks == ss.DesiredTasks:
		return "running", nil
	default:
		return "error", nil
	}
}

// Event wraps typed events from Docker's /events stream.
type Event struct {
	Type    string          `json:"type"`    // "service", "container"
	Service string          `json:"service"` // service name for filtering
	Time    time.Time       `json:"time"`    // event timestamp
	Payload json.RawMessage `json:"payload"` // ServiceEvent or ContainerEvent
}

// ServiceEvent represents a swarm service state change.
type ServiceEvent struct {
	Action      string `json:"action"`
	UpdateState string `json:"update_state,omitempty"`
}

// ContainerEvent represents a container lifecycle event.
type ContainerEvent struct {
	Action   string `json:"action"`
	TaskID   string `json:"task_id"`
	TaskName string `json:"task_name"`
	Image    string `json:"image"`
	ExitCode string `json:"exit_code,omitempty"`
	ExecDur  string `json:"exec_dur,omitempty"`
}

// swarmEvent is the Docker API event structure.
type swarmEvent struct {
	Type   string `json:"Type"`
	Action string `json:"Action"`
	Actor  struct {
		ID         string            `json:"ID"`
		Attributes map[string]string `json:"Attributes"`
	} `json:"Actor"`
	Time int64 `json:"time"`
}

// ToEvent converts a Docker event to our Event type.
func (se *swarmEvent) ToEvent() (event Event, err error) {

	attrs := se.Actor.Attributes
	event.Time = time.Unix(se.Time, 0)

	switch se.Type {
	case "service":
		event.Type = "service"
		event.Service = attrs["name"]
		event.Payload, err = json.Marshal(ServiceEvent{
			Action:      se.Action,
			UpdateState: attrs["updatestate.new"],
		})
		if err != nil {
			err = errors.Wrap(err, "marshal service event")
		}

	case "container":
		event.Type = "container"
		event.Service = attrs["com.docker.swarm.service.name"]
		event.Payload, err = json.Marshal(ContainerEvent{
			Action:   se.Action,
			TaskID:   attrs["com.docker.swarm.task.id"],
			TaskName: attrs["com.docker.swarm.task.name"],
			Image:    attrs["image"],
			ExitCode: attrs["exitCode"],
			ExecDur:  attrs["execDuration"],
		})
		if err != nil {
			err = errors.Wrap(err, "marshal container event")
		}

	default:
		err = errors.Errorf("unsupported event type: %s", se.Type)
	}

	return
}

// Events streams Docker service and container events.
func (d *Swarm) Events(ctx context.Context) (<-chan Event, error) {

	lines, err := d.client.StreamLines(ctx, "/v1.52/events")
	if err != nil {
		return nil, err
	}

	events := make(chan Event)
	go func() {
		defer close(events)

		for data := range lines {

			var se swarmEvent
			err := json.Unmarshal(data, &se)
			if err != nil {
				err = errors.Wrap(err, "unmarshal swarm event")
				d.logger.Error(ctx, "failed to unmarshal event", err)
				continue
			}

			if se.Type != "service" && se.Type != "container" {
				continue
			}

			event, err := se.ToEvent()
			if err != nil {
				d.logger.Error(ctx, "failed to convert event", err)
				continue
			}

			select {
			case events <- event:
			case <-ctx.Done():
				return
			}
		}
	}()

	return events, nil
}
