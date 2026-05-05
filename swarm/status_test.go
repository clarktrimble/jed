package swarm_test

import (
	"context"

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

	Describe("Status", func() {
		Context("running service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(1, 1, "completed", ""), rcv)
					return nil
				}
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
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(0, 0, "completed", ""), rcv)
					return nil
				}
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
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(0, 1, "completed", ""), rcv)
					return nil
				}
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
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(1, 0, "updating", "update in progress"), rcv)
					return nil
				}
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
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(1, 0, "", ""), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusError))
			})
		})

		Context("deploy failed (paused)", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(1, 0, "paused", "update paused due to failure"), rcv)
					return nil
				}
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
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(2, 1, "completed", "update completed"), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, err = sw.Status(ctx, "myservice")
			})

			It("should return error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusError))
			})
		})
	})
})
