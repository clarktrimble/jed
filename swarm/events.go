package swarm

import (
	"context"
	"encoding/json"
	"time"

	"github.com/pkg/errors"
)

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
