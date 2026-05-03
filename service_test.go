package jed_test

import (
	"encoding/json"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
)

func TestJed(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jed Suite")
}

var _ = Describe("Service", func() {
	Describe("Services.Find", func() {
		services := jed.Services{
			{Name: "postgres", Image: "postgres:16"},
			{Name: "redis", Image: "redis:7"},
		}

		It("finds a service by name", func() {
			svc, err := services.Find("redis")
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.Image).To(Equal("redis:7"))
		})

		It("returns an error when missing", func() {
			_, err := services.Find("missing")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("service missing not found"))
		})
	})

	Describe("JSON", func() {
		It("uses lower-case field names for name and image", func() {
			data, err := json.Marshal(jed.Service{Name: "app", Image: "app:v1", Network: "svc-net"})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring(`"name":"app"`))
			Expect(string(data)).To(ContainSubstring(`"image":"app:v1"`))
			Expect(string(data)).NotTo(ContainSubstring(`"Name"`))
			Expect(string(data)).NotTo(ContainSubstring(`"Image"`))
		})
	})

	Describe("Validate", func() {
		It("accepts a minimal service", func() {
			svc := jed.Service{Name: "app", Image: "app:v1", Network: "svc-net"}
			Expect(svc.Validate()).To(Succeed())
		})

		It("reports missing required fields", func() {
			svc := jed.Service{}
			err := svc.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("image is required"))
			Expect(err.Error()).To(ContainSubstring("name is required"))
			Expect(err.Error()).To(ContainSubstring("network is required"))
		})

		It("rejects invalid restart policy", func() {
			svc := jed.Service{
				Name:    "app",
				Image:   "app:v1",
				Network: "svc-net",
				Restart: jed.RestartPolicy{Condition: "sometimes"},
			}
			err := svc.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("restart condition"))
		})

		It("rejects negative restart attempts", func() {
			attempts := -1
			svc := jed.Service{
				Name:    "app",
				Image:   "app:v1",
				Network: "svc-net",
				Restart: jed.RestartPolicy{Condition: jed.RestartOnFailure, MaxAttempts: &attempts},
			}
			err := svc.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("restart max_attempts cannot be negative"))
		})
	})
})
