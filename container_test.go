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

var _ = Describe("Container", func() {
	var (
		cfg    *jed.Config
		client *ClientMock
		lgr    *LoggerMock
		ctx    context.Context
		svc    *jed.Jed
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
		svc, err = cfg.NewJed(ctx, client, lgr, os.DirFS("test/data/cntr-cfg"))
		Expect(err).NotTo(HaveOccurred())
	})

	Describe("Containers", func() {
		var (
			containers jed.Containers
		)

		JustBeforeEach(func() {
			containers, err = svc.Containers(ctx)
		})

		When("multiple containers are running", func() {
			BeforeEach(func() {
				statuses := []jed.Container{
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
				client.SendObjectFunc = mockContainers(statuses)
			})

			It("should return containers without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).NotTo(BeNil())
			})

			It("should map by service name with suffix stripped", func() {
				Expect(containers).To(HaveLen(3))
				Expect(containers).To(HaveKey("postgres"))
				Expect(containers).To(HaveKey("redis"))
				Expect(containers).To(HaveKey("nginx"))
			})

			It("should preserve full container details", func() {
				Expect(containers["postgres"].Id).To(Equal("abc123"))
				Expect(containers["postgres"].Names).To(Equal([]string{"/postgres-x7y9z2n"}))
				Expect(containers["postgres"].Image).To(Equal("postgres:14"))
				Expect(containers["postgres"].State).To(Equal("running"))

				Expect(containers["redis"].Id).To(Equal("def456"))
				Expect(containers["nginx"].Id).To(Equal("ghi789"))
			})
		})

		When("no containers exist", func() {
			BeforeEach(func() {
				client.SendObjectFunc = mockContainers([]jed.Container{})
			})

			It("should return empty containers without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(HaveLen(0))
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

	Describe("Containers.DeployName", func() {
		var (
			containers jed.Containers
			deployName string
		)

		BeforeEach(func() {
			statuses := []jed.Container{
				{
					Id:     "abc123",
					Names:  []string{"/postgres-x7y9z2n"},
					State:  "running",
					Labels: map[string]string{"managed_by": "jed"},
				},
			}
			client.SendObjectFunc = mockContainers(statuses)
			containers, err = svc.Containers(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			deployName, err = containers.DeployName("postgres")
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
				deployName, err = containers.DeployName("nonexistent")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("container nonexistent not found"))
			})
		})
	})

	Describe("Containers.Id", func() {
		var (
			containers jed.Containers
			id         string
		)

		BeforeEach(func() {
			statuses := []jed.Container{
				{
					Id:     "abc123def456",
					Names:  []string{"/postgres-x7y9z2n"},
					State:  "running",
					Labels: map[string]string{"managed_by": "jed"},
				},
			}
			client.SendObjectFunc = mockContainers(statuses)
			containers, err = svc.Containers(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			id, err = containers.Id("postgres")
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
				id, err = containers.Id("nonexistent")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("container nonexistent not found"))
			})
		})
	})
})
