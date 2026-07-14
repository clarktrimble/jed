package swarm_test

import (
	"context"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Status", func() {
	var (
		client *ClientMock
		sw     *swarm.Swarm
		ctx    context.Context
		err    error
		status swarm.Status
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &ClientMock{}
		sw = swarm.New(client, nopLogger{})
	})

	serviceWith := func(desired, running int, updateState, updateMessage string) []map[string]any {
		return []map[string]any{{
			"ServiceStatus": map[string]any{
				"DesiredTasks": desired,
				"RunningTasks": running,
			},
			"UpdateStatus": map[string]any{
				"State":   updateState,
				"Message": updateMessage,
			},
		}}
	}

	tasksWith := func(states ...string) []map[string]any {
		tasks := make([]map[string]any, len(states))
		for i, state := range states {
			tasks[i] = map[string]any{
				"Status": map[string]any{"State": state},
			}
		}
		return tasks
	}

	// respond routes /tasks requests to the task data and everything else to
	// the service data, mirroring the two calls Status makes.
	respond := func(svc, tasks []map[string]any) func(context.Context, string, string, any, any) error {
		return func(ctx context.Context, method, path string, snd, rcv any) error {
			if strings.Contains(path, "/tasks") {
				mockResponse(tasks, rcv)
			} else {
				mockResponse(svc, rcv)
			}
			return nil
		}
	}

	Describe("Status", func() {
		Context("running service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(1, 1, "completed", ""), tasksWith("running"))
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return running", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusRunning))
			})
		})

		Context("stopped service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(0, 0, "completed", ""), nil)
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return stopped", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusStopped))
			})
		})

		Context("stopping service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(0, 1, "completed", ""), tasksWith("running"))
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return pending", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusPending))
			})
		})

		Context("deploying service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(1, 0, "updating", "update in progress"), tasksWith("starting"))
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return pending", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusPending))
			})
		})

		Context("running < desired with no update state", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(1, 0, "", ""), tasksWith("preparing"))
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return pending", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusPending))
			})
		})

		Context("deploy failed (paused)", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(1, 0, "paused", "update paused due to failure"), nil)
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusError))
			})
		})

		Context("task mismatch after completed update", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(2, 1, "completed", "update completed"), tasksWith("running", "starting"))
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return pending", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusPending))
			})
		})

		Context("rejected task, nothing trying", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(1, 0, "", ""), tasksWith("rejected", "rejected"))
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusError))
			})
		})

		Context("old rejected task but a new one trying", func() {
			BeforeEach(func() {
				client.SendObjectFunc = respond(serviceWith(1, 0, "", ""), tasksWith("rejected", "preparing"))
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return pending", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusPending))
			})
		})
	})

	Describe("Statuses", func() {
		var statuses map[string]swarm.Status

		BeforeEach(func() {
			svcs := []map[string]any{
				{
					"ID":            "svc-good",
					"Spec":          map[string]any{"Name": "good"},
					"ServiceStatus": map[string]any{"DesiredTasks": 1, "RunningTasks": 1},
					"UpdateStatus":  map[string]any{"State": "completed"},
				},
				{
					"ID":            "svc-bad",
					"Spec":          map[string]any{"Name": "bad"},
					"ServiceStatus": map[string]any{"DesiredTasks": 1, "RunningTasks": 0},
					"UpdateStatus":  map[string]any{"State": ""},
				},
			}
			tasks := []map[string]any{
				{"ServiceID": "svc-good", "Status": map[string]any{"State": "running"}},
				{"ServiceID": "svc-bad", "Status": map[string]any{"State": "rejected"}},
			}
			client.SendObjectFunc = respond(svcs, tasks)
		})

		JustBeforeEach(func() {
			statuses, err = sw.Statuses(ctx)
		})

		It("groups tasks by service and reports each status", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(statuses).To(Equal(map[string]swarm.Status{
				"good": swarm.StatusRunning,
				"bad":  swarm.StatusError,
			}))
		})
	})
})
