package swarm_test

import (
	"context"
	"encoding/json"
	"errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Service", func() {
	var (
		client   *ClientMock
		sw       *swarm.Swarm
		ctx      context.Context
		err      error
		svcInfo  *swarm.ServiceInfo
		services []swarm.Service
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &ClientMock{}
		sw = swarm.New(client, nopLogger{})
	})

	Describe("ListServices", func() {
		BeforeEach(func() {
			testData := loadTestData("get-services.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				return json.Unmarshal(testData, rcv)
			}
		})

		JustBeforeEach(func() {
			services, err = sw.ListServices(ctx)
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should parse services", func() {
			Expect(services).To(HaveLen(4))
			Expect(services[0].ID).To(Equal("dhgtqt27zv3zf6n13g2zgp3b0"))
			Expect(services[0].Name).To(Equal("whoami"))
			Expect(services[3].Name).To(Equal("tag"))
		})
	})

	Describe("GetService not found", func() {
		BeforeEach(func() {
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				mockResponse([]map[string]any{}, rcv)
				return nil
			}
		})

		JustBeforeEach(func() {
			_, err = sw.GetService(ctx, "missing")
		})

		It("should return ErrServiceNotFound", func() {
			Expect(err).To(HaveOccurred())
			Expect(errors.Is(err, swarm.ErrServiceNotFound)).To(BeTrue())
			Expect(err.Error()).To(ContainSubstring("missing"))
		})
	})

	Describe("GetService", func() {
		BeforeEach(func() {
			testData := loadTestData("get-services-filtered.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				err := json.Unmarshal(testData, rcv)
				Expect(err).NotTo(HaveOccurred())
				return nil
			}
		})

		JustBeforeEach(func() {
			svcInfo, err = sw.GetService(ctx, "tag")
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should parse Version", func() {
			Expect(svcInfo.Version.Index).To(BeNumerically(">", 0))
		})

		It("should parse Spec", func() {
			Expect(svcInfo.Spec.Name).To(Equal("tag"))
			Expect(svcInfo.Spec.TaskTemplate.ForceUpdate).To(Equal(uint64(0)))
			Expect(svcInfo.Spec.TaskTemplate.ContainerSpec).NotTo(BeEmpty())
		})

		It("should parse Endpoint", func() {
			Expect(svcInfo.Endpoint.Ports).To(HaveLen(1))
			Expect(svcInfo.Endpoint.Ports[0].TargetPort).To(Equal(3031))
			Expect(svcInfo.Endpoint.Ports[0].PublishedPort).To(Equal(8010))
			Expect(svcInfo.Endpoint.VirtualIPs).To(HaveLen(2))
		})

		It("should parse UpdateStatus", func() {
			Expect(svcInfo.UpdateStatus.State).To(Equal("completed"))
			Expect(svcInfo.UpdateStatus.CompletedAt).NotTo(BeZero())
		})

		It("should parse timestamps", func() {
			Expect(svcInfo.CreatedAt).NotTo(BeZero())
			Expect(svcInfo.UpdatedAt).NotTo(BeZero())
			Expect(svcInfo.UpdatedAt.After(svcInfo.CreatedAt)).To(BeTrue())
		})
	})

	Describe("DeleteService", func() {
		var calledPath string

		BeforeEach(func() {
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				calledPath = path
				return nil
			}
		})

		JustBeforeEach(func() {
			err = sw.DeleteService(ctx, "my-service")
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call DELETE on correct path", func() {
			Expect(calledPath).To(Equal("/v1.52/services/my-service"))
		})
	})
})
