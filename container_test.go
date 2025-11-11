package jed_test

import (
	"context"
	"io"
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
		store  *StoreMock
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
		// Use mock store instead of bbolt for faster, isolated tests
		store = loadMockStoreFromFS("test/data/svc-cfg")
		svc, err = cfg.New(ctx, client, lgr, store)
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
				client.SendObjectFunc = mockContainers(testContainerList())
			})

			It("should return containers without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).NotTo(BeNil())
			})

			It("should contain containers with service names parsed", func() {
				Expect(containers).To(HaveLen(3))

				services := make(map[string]bool)
				for _, c := range containers {
					services[c.ServiceName] = true
				}
				Expect(services).To(HaveKey("postgres"))
				Expect(services).To(HaveKey("redis"))
				Expect(services).To(HaveKey("nginx"))
			})

			It("should preserve full container details", func() {
				postgres, err := containers.Find("postgres")
				Expect(err).NotTo(HaveOccurred())
				Expect(postgres.Id).To(Equal("abc123"))
				Expect(postgres.Names).To(Equal([]string{"/postgres-x7y9z2n"}))
				Expect(postgres.Image).To(Equal("postgres:14"))
				Expect(postgres.State).To(Equal("running"))

				redis, err := containers.Find("redis")
				Expect(err).NotTo(HaveOccurred())
				Expect(redis.Id).To(Equal("def456"))

				nginx, err := containers.Find("nginx")
				Expect(err).NotTo(HaveOccurred())
				Expect(nginx.Id).To(Equal("ghi789"))
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

		When("container name doesn't match suffix pattern", func() {
			BeforeEach(func() {
				cntrs := []jed.Container{
					{
						Id:     "abc123",
						Names:  []string{"/postgres"},
						Image:  "postgres:14",
						State:  "running",
						Status: "Up 2 hours",
						Labels: map[string]string{"managed_by": "jed"},
					},
				}
				client.SendObjectFunc = mockContainers(cntrs)
			})

			It("should skip non-matching containers and log error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(HaveLen(0))
				Expect(lgr.ErrorCalls()).To(HaveLen(1))
				Expect(lgr.ErrorCalls()[0].Msg).To(Equal("ignoring managed_by=jed containers"))
				Expect(lgr.ErrorCalls()[0].Err.Error()).To(ContainSubstring("unexpected containers"))
			})
		})
	})

	Describe("Containers.Find", func() {
		var (
			containers jed.Containers
			ctr        jed.Container
		)

		BeforeEach(func() {
			cntrs := testContainerList()
			client.SendObjectFunc = mockContainers(cntrs[:1])
			containers, err = svc.Containers(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			ctr, err = containers.Find("postgres")
		})

		When("container exists", func() {
			It("should return container without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(ctr.Id).To(Equal("abc123"))
				Expect(ctr.ServiceName).To(Equal("postgres"))
			})
		})

		When("container not found", func() {
			JustBeforeEach(func() {
				ctr, err = containers.Find("nonexistent")
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("container nonexistent not found"))
			})
		})
	})

	Describe("Container.DeployName", func() {
		var (
			ctr        jed.Container
			deployName string
		)

		BeforeEach(func() {
			ctr = jed.Container{
				Names: []string{"/postgres-x7y9z2n"},
			}
		})

		JustBeforeEach(func() {
			deployName = ctr.DeployName()
		})

		When("container has name with leading slash", func() {
			It("should return deploy name without leading slash", func() {
				Expect(deployName).To(Equal("postgres-x7y9z2n"))
			})
		})
	})
})
