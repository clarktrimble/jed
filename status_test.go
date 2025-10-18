package jed_test

import (
	"context"
	"io"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
)

var _ = Describe("Status", func() {
	var (
		cfg    *jed.Config
		client *ClientMock
		lgr    *LoggerMock
		ctx    context.Context
		svc    *jed.Svc
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()
		cfg = &jed.Config{}
		lgr = &LoggerMock{
			InfoFunc:  func(ctx context.Context, msg string, kv ...any) {},
			DebugFunc: func(ctx context.Context, msg string, kv ...any) {},
			ErrorFunc: func(ctx context.Context, msg string, err error, kv ...any) {},
		}
		client = &ClientMock{
			// Default: mock checkImage to always succeed
			SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
				if method == "GET" && strings.Contains(path, "/images/") {
					return nil
				}
				return nil
			},
		}
		svc, err = cfg.NewSvc(ctx, client, lgr, os.DirFS("test/data/cntr-cfg"))
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Statii", func() {
		var (
			statii jed.Statii
		)

		JustBeforeEach(func() {
			statii, err = svc.Statii(ctx)
		})

		When("multiple containers are running", func() {
			BeforeEach(func() {
				statuses := []jed.Status{
					{
						Id:     "abc123",
						Names:  []string{"/postgres-x7y9z2n"},
						Image:  "postgres:14",
						State:  "running",
						Status: "Up 2 hours",
						Labels: map[string]string{"managed_by": "jed"},
					},
					{
						Id:     "def456",
						Names:  []string{"/redis-k3m5p1q"},
						Image:  "redis:7",
						State:  "running",
						Status: "Up 1 hour",
						Labels: map[string]string{"managed_by": "jed"},
					},
					{
						Id:     "ghi789",
						Names:  []string{"/nginx-w8x2y4z"},
						Image:  "nginx:latest",
						State:  "exited",
						Status: "Exited (0) 5 minutes ago",
						Labels: map[string]string{"managed_by": "jed"},
					},
				}
				client.SendObjectFunc = mockStatii(statuses)
			})

			It("should return statii without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(statii).NotTo(BeNil())
			})

			It("should map by base name with suffix stripped", func() {
				Expect(statii).To(HaveLen(3))
				Expect(statii).To(HaveKey("postgres"))
				Expect(statii).To(HaveKey("redis"))
				Expect(statii).To(HaveKey("nginx"))
			})

			It("should preserve full status details", func() {
				Expect(statii["postgres"].Id).To(Equal("abc123"))
				Expect(statii["postgres"].Names).To(Equal([]string{"/postgres-x7y9z2n"}))
				Expect(statii["postgres"].Image).To(Equal("postgres:14"))
				Expect(statii["postgres"].State).To(Equal("running"))

				Expect(statii["redis"].Id).To(Equal("def456"))
				Expect(statii["nginx"].Id).To(Equal("ghi789"))
			})
		})

		When("no containers exist", func() {
			BeforeEach(func() {
				client.SendObjectFunc = mockStatii([]jed.Status{})
			})

			It("should return empty statii without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(statii).To(HaveLen(0))
			})
		})

		When("Docker API fails", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					if method == "GET" && strings.Contains(path, "/containers/json") {
						return io.ErrUnexpectedEOF
					}
					return nil
				}
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(io.ErrUnexpectedEOF))
			})
		})
	})

	Describe("Statii.DeployName", func() {
		var (
			statii     jed.Statii
			deployName string
		)

		BeforeEach(func() {
			statuses := []jed.Status{
				{
					Id:     "abc123",
					Names:  []string{"/postgres-x7y9z2n"},
					State:  "running",
					Labels: map[string]string{"managed_by": "jed"},
				},
			}
			client.SendObjectFunc = mockStatii(statuses)
			statii, err = svc.Statii(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			deployName, err = statii.DeployName("postgres")
		})

		When("container exists", func() {
			It("should return deploy name without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(deployName).To(Equal("postgres-x7y9z2n"))
			})
		})

		When("container not found", func() {
			BeforeEach(func() {
				deployName = ""
			})

			JustBeforeEach(func() {
				deployName, err = statii.DeployName("nonexistent")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("container nonexistent not found"))
			})
		})
	})

	Describe("Statii.Id", func() {
		var (
			statii jed.Statii
			id     string
		)

		BeforeEach(func() {
			statuses := []jed.Status{
				{
					Id:     "abc123def456",
					Names:  []string{"/postgres-x7y9z2n"},
					State:  "running",
					Labels: map[string]string{"managed_by": "jed"},
				},
			}
			client.SendObjectFunc = mockStatii(statuses)
			statii, err = svc.Statii(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			id, err = statii.Id("postgres")
		})

		When("container exists", func() {
			It("should return container ID without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal("abc123def456"))
			})
		})

		When("container not found", func() {
			BeforeEach(func() {
				id = ""
			})

			JustBeforeEach(func() {
				id, err = statii.Id("nonexistent")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("container nonexistent not found"))
			})
		})
	})
})
