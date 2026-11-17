package swarm

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

// Status represents the operational state of a service.
type Status string

const (
	// StatusRunning means all desired service tasks are running.
	StatusRunning Status = "running"
	// StatusStopped means a service has no desired or running tasks.
	StatusStopped Status = "stopped"
	// StatusExited means replicas are desired, no task is active, and the latest task completed cleanly.
	StatusExited Status = "exited"
	// StatusPending means a service is deploying, updating, stopping, or converging.
	StatusPending Status = "pending"
	// StatusError means an update or rollback failed, or no task is active and
	// the latest task failed or was rejected.
	StatusError Status = "error"
)

// ServiceStatus holds task counts for a service.
// Populated when GetService is called (requires ?status=true).
type ServiceStatus struct {
	RunningTasks   int `json:"RunningTasks"`
	DesiredTasks   int `json:"DesiredTasks"`
	CompletedTasks int `json:"CompletedTasks"`
}

// UpdateStatus represents the status of a service update.
type UpdateStatus struct {
	State       string    `json:"State"`
	Message     string    `json:"Message"`
	StartedAt   time.Time `json:"StartedAt"`
	CompletedAt time.Time `json:"CompletedAt"`
}

// Statuses returns the status of all services.
func (d *Swarm) Statuses(ctx context.Context) (map[string]Status, error) {

	var svcs []serviceListItem
	err := d.client.SendObject(ctx, "GET", "/v1.52/services?status=true", nil, &svcs)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list services")
	}

	// Tasks distinguish converging services from failed or cleanly exited ones.
	// This call could be skipped when every service's status is already decided
	// by counts and update state alone.
	var tasks []taskResponse
	err = d.client.SendObject(ctx, "GET", "/v1.52/tasks", nil, &tasks)
	if err != nil {
		return nil, errors.Wrap(err, "failed to list tasks")
	}

	byService := make(map[string][]Task, len(svcs))
	for _, t := range tasks {
		byService[t.ServiceID] = append(byService[t.ServiceID], Task{
			State:     t.Status.State,
			Timestamp: t.Status.Timestamp,
		})
	}

	result := make(map[string]Status, len(svcs))
	for _, s := range svcs {
		result[s.Spec.Name] = computeStatus(s.ServiceStatus, s.UpdateStatus, byService[s.ID])
	}

	return result, nil
}

// Status returns the current status of a deployed service.
//
// Returns status:
//   - StatusStopped - DesiredTasks == 0 and no tasks running
//   - StatusPending - transitioning, converging, deploying, or stopping
//   - StatusError   - deploy/rollback failed (paused), or latest task failed/rejected with none active
//   - StatusRunning - healthy, all desired tasks running
//   - StatusExited  - replicas desired, no active task, latest task completed cleanly
//
// Note: Job mode services (replicated-job, global-job) would need different logic.
// Todo: consider rollback, how hard will this be to support from a ux sanity perspective?
func (d *Swarm) Status(ctx context.Context, name string) (Status, error) {

	svc, err := d.GetService(ctx, name)
	if err != nil {
		return "", err
	}

	tasks, err := d.ServiceTasks(ctx, name)
	if err != nil {
		return "", err
	}

	return computeStatus(svc.ServiceStatus, svc.UpdateStatus, tasks), nil
}

// computeStatus determines the status from ServiceStatus, UpdateStatus, and tasks.
func computeStatus(ss ServiceStatus, us UpdateStatus, tasks []Task) Status {
	switch {
	case ss.DesiredTasks == 0 && ss.RunningTasks == 0:
		return StatusStopped
	case ss.DesiredTasks == 0 && ss.RunningTasks > 0:
		return StatusPending // stopping
	case us.State == "updating" || us.State == "rollback_started":
		return StatusPending // deploying or rolling back
	case us.State == "paused" || us.State == "rollback_paused":
		return StatusError // deploy or rollback failed
	case ss.RunningTasks == ss.DesiredTasks:
		return StatusRunning
	}

	var latest Task
	var latestAt time.Time
	for i, t := range tasks {
		if isActive(t.State) {
			return StatusPending // still converging, regardless of task history
		}

		timestamp, err := time.Parse(time.RFC3339Nano, t.Timestamp)
		if err != nil {
			timestamp = time.Time{}
		}
		// Missing or malformed timestamps count as zero; ties keep the first task.
		if i == 0 || timestamp.After(latestAt) {
			latest = t
			latestAt = timestamp
		}
	}

	switch latest.State {
	case "failed", "rejected":
		return StatusError
	case "complete":
		return StatusExited
	default:
		return StatusPending // no tasks yet, or no conclusive outcome
	}
}

// isActive reports whether a task state is working toward running.
func isActive(state string) bool {
	switch state {
	case "new", "pending", "assigned", "accepted", "preparing", "ready", "starting", "running":
		return true
	default:
		return false
	}
}
