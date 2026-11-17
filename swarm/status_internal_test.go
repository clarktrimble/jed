package swarm

import "testing"

func TestComputeStatus(t *testing.T) {
	const (
		older = "2026-11-17T12:00:00.9Z"
		newer = "2026-11-17T12:00:00.91Z"
	)
	wanted := ServiceStatus{DesiredTasks: 1}
	completed := []Task{{State: "complete", Timestamp: newer}}

	type statusCase struct {
		name  string
		ss    ServiceStatus
		us    UpdateStatus
		tasks []Task
		want  Status
	}
	tests := []statusCase{
		{name: "stopped despite completed task", tasks: completed, want: StatusStopped},
		{name: "stopped despite paused update", us: UpdateStatus{State: "paused"}, tasks: completed, want: StatusStopped},
		{name: "stopping", ss: ServiceStatus{RunningTasks: 1}, tasks: completed, want: StatusPending},
		{name: "stopping despite paused update", ss: ServiceStatus{RunningTasks: 1}, us: UpdateStatus{State: "paused"}, want: StatusPending},
		{name: "running despite completed history", ss: ServiceStatus{DesiredTasks: 1, RunningTasks: 1}, tasks: completed, want: StatusRunning},
		{name: "partly running", ss: ServiceStatus{DesiredTasks: 2, RunningTasks: 1}, tasks: []Task{{State: "running"}, {State: "complete"}}, want: StatusPending},
		{name: "no tasks", ss: wanted, want: StatusPending},
		{name: "empty task list", ss: wanted, tasks: []Task{}, want: StatusPending},
	}
	for _, state := range []string{"updating", "rollback_started"} {
		tests = append(tests, statusCase{
			name: state + " precedes task completion and running counts",
			ss:   ServiceStatus{DesiredTasks: 1, RunningTasks: 1},
			us:   UpdateStatus{State: state}, tasks: completed, want: StatusPending,
		})
	}
	for _, state := range []string{"paused", "rollback_paused"} {
		tests = append(tests, statusCase{
			name: state + " precedes task completion and running counts",
			ss:   ServiceStatus{DesiredTasks: 1, RunningTasks: 1},
			us:   UpdateStatus{State: state}, tasks: completed, want: StatusError,
		})
	}
	for _, state := range []string{"new", "pending", "assigned", "accepted", "preparing", "ready", "starting", "running"} {
		active := Task{State: state, Timestamp: older}
		terminal := []Task{{State: "failed", Timestamp: newer}, {State: "complete", Timestamp: newer}}
		tests = append(tests,
			statusCase{name: state + " active first", ss: wanted, tasks: append([]Task{active}, terminal...), want: StatusPending},
			statusCase{name: state + " active last", ss: wanted, tasks: append(terminal, active), want: StatusPending},
		)
	}
	for _, tc := range []struct {
		state string
		want  Status
	}{
		{"complete", Status("exited")},
		{"failed", StatusError},
		{"rejected", StatusError},
		{"shutdown", StatusPending},
		{"orphaned", StatusPending},
		{"unknown", StatusPending},
	} {
		tests = append(tests, statusCase{
			name: "single " + tc.state, ss: wanted,
			tasks: []Task{{State: tc.state, Timestamp: newer}}, want: tc.want,
		})
	}

	// Every pair runs in both response orders: chronology must come from the
	// timestamp, not the task list's order or its textual timestamp order.
	for _, tc := range []struct {
		name   string
		first  Task
		second Task
		want   Status
	}{
		{"failed then complete", Task{State: "failed", Timestamp: older}, Task{State: "complete", Timestamp: newer}, Status("exited")},
		{"rejected then complete", Task{State: "rejected", Timestamp: older}, Task{State: "complete", Timestamp: newer}, Status("exited")},
		{"complete then failed", Task{State: "complete", Timestamp: older}, Task{State: "failed", Timestamp: newer}, StatusError},
		{"complete then rejected", Task{State: "complete", Timestamp: older}, Task{State: "rejected", Timestamp: newer}, StatusError},
		{"failed then shutdown", Task{State: "failed", Timestamp: older}, Task{State: "shutdown", Timestamp: newer}, StatusPending},
		{"whole second then fractional", Task{State: "complete", Timestamp: "2026-11-17T12:00:00Z"}, Task{State: "failed", Timestamp: "2026-11-17T12:00:00.1Z"}, StatusError},
		{"time zone offset", Task{State: "failed", Timestamp: "2026-11-17T13:00:00+01:00"}, Task{State: "complete", Timestamp: "2026-11-17T12:00:00.1Z"}, Status("exited")},
		{"missing timestamp", Task{State: "failed"}, Task{State: "complete", Timestamp: newer}, Status("exited")},
		{"malformed timestamp", Task{State: "failed", Timestamp: "not-a-time"}, Task{State: "complete", Timestamp: newer}, Status("exited")},
	} {
		tests = append(tests,
			statusCase{name: tc.name + " oldest first", ss: wanted, tasks: []Task{tc.first, tc.second}, want: tc.want},
			statusCase{name: tc.name + " newest first", ss: wanted, tasks: []Task{tc.second, tc.first}, want: tc.want},
		)
	}
	for _, tc := range []struct {
		name  string
		tasks []Task
		want  Status
	}{
		{"missing timestamps keep first completion", []Task{{State: "complete"}, {State: "failed"}}, Status("exited")},
		{"malformed timestamps keep first completion", []Task{{State: "complete", Timestamp: "bad"}, {State: "failed", Timestamp: "worse"}}, Status("exited")},
		{"missing and malformed timestamps tie", []Task{{State: "complete"}, {State: "failed", Timestamp: "bad"}}, Status("exited")},
		{"equal instants keep first completion", []Task{{State: "complete", Timestamp: older}, {State: "failed", Timestamp: "2026-11-17T12:00:00.900Z"}}, Status("exited")},
		{"equal instants keep first failure", []Task{{State: "failed", Timestamp: older}, {State: "complete", Timestamp: "2026-11-17T12:00:00.900Z"}}, StatusError},
	} {
		tests = append(tests, statusCase{name: tc.name, ss: wanted, tasks: tc.tasks, want: tc.want})
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := computeStatus(tt.ss, tt.us, tt.tasks); got != tt.want {
				t.Errorf("computeStatus() = %q, want %q", got, tt.want)
			}
		})
	}
}
