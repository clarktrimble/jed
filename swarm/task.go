package swarm

import (
	"bytes"
	"context"
	"fmt"
	"io"

	"github.com/clarktrimble/jed/internal/dockerlog"
	"github.com/pkg/errors"
)

// Task represents a service task.
type Task struct {
	ID        string
	State     string
	Error     string
	Image     string
	Timestamp string
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

// TaskLogs retrieves logs from a swarm task.
func (d *Swarm) TaskLogs(ctx context.Context, taskID, tail string) ([]byte, error) {

	path := fmt.Sprintf("/v1.52/tasks/%s/logs?stdout=true&stderr=true&tail=%s", taskID, tail)

	rawLogs, err := d.client.SendJson(ctx, "GET", path, nil)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to get logs for task %q", taskID)
	}

	reader := dockerlog.Decode(bytes.NewReader(rawLogs))
	logs, err := io.ReadAll(reader)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to decode logs for task %q", taskID)
	}

	return logs, nil
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
