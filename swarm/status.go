package swarm

import (
	"context"
	"time"

	"github.com/pkg/errors"
)

// Status represents the operational state of a service.
type Status string

const (
	StatusRunning Status = "running"
	StatusStopped Status = "stopped"
	StatusPending Status = "pending"
	StatusError   Status = "error"
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

	result := make(map[string]Status, len(svcs))
	for _, s := range svcs {
		result[s.Spec.Name] = computeStatus(s.ServiceStatus, s.UpdateStatus)
	}

	return result, nil
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
