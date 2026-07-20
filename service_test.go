package jed_test

import (
	"encoding/json"
	"fmt"
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
		It("uses lower-case field names for name, enabled, and image", func() {
			data, err := json.Marshal(jed.Service{Name: "app", Enabled: true, Image: "app:v1", Network: "svc-net"})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring(`"name":"app"`))
			Expect(string(data)).To(ContainSubstring(`"enabled":true`))
			Expect(string(data)).To(ContainSubstring(`"image":"app:v1"`))
			Expect(string(data)).NotTo(ContainSubstring(`"Name"`))
			Expect(string(data)).NotTo(ContainSubstring(`"Enabled"`))
			Expect(string(data)).NotTo(ContainSubstring(`"Image"`))
		})

		It("accepts restart as a string", func() {
			var svc jed.Service
			err := json.Unmarshal([]byte(`{"name":"app","image":"app:v1","network":"svc-net","restart":"on-failure"}`), &svc)
			Expect(err).NotTo(HaveOccurred())
			Expect(svc.Restart).To(Equal(jed.RestartOnFailure))
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

		It("rejects disabled services with replicas", func() {
			svc := jed.Service{Name: "app", Image: "app:v1", Network: "svc-net", Replicas: 1}
			err := svc.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("disabled service cannot have replicas"))
		})

		It("rejects invalid restart policy", func() {
			svc := jed.Service{
				Name:    "app",
				Image:   "app:v1",
				Network: "svc-net",
				Restart: "sometimes",
			}
			err := svc.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("restart"))
		})

		It("allows numeric and templated user values before render", func() {
			for _, user := range []string{"1000", "1000:967", "1000:{{DOCKER_GID}}", "{{DOCKER_UID}}:{{DOCKER_GID}}", "{{DOCKER_USER}}"} {
				svc := jed.Service{Name: "app", Image: "app:v1", Network: "svc-net", User: user}
				Expect(svc.Validate()).To(Succeed(), user)
			}
		})

		It("rejects malformed user templates", func() {
			for _, user := range []string{"1000:{{DOCKER_GID", "1000:{{}}", "1000:{{DOCKER-GID}}", "1000:docker", "1000:", ":967", "1:2:3", "-1", "+1"} {
				svc := jed.Service{Name: "app", Image: "app:v1", Network: "svc-net", User: user}
				Expect(svc.Validate()).NotTo(Succeed(), user)
			}
		})

		It("allows numeric and templated groups before render", func() {
			for _, groups := range [][]string{{"967"}, {"{{DOCKER_GID}}"}, {"967", "{{DOCKER_GID}}"}} {
				svc := jed.Service{Name: "app", Image: "app:v1", Network: "svc-net", Groups: groups}
				Expect(svc.Validate()).To(Succeed(), fmt.Sprint(groups))
			}
		})

		It("rejects malformed groups", func() {
			for _, group := range []string{"", "967:968", "{{DOCKER_GID", "{{}}", "{{DOCKER-GID}}", "docker", "-1"} {
				svc := jed.Service{Name: "app", Image: "app:v1", Network: "svc-net", Groups: []string{group}}
				Expect(svc.Validate()).NotTo(Succeed(), group)
			}
		})
	})
})
