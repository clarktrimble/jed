package swarm_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Config", func() {
	It("returns an error when config is nil", func() {
		_, err := (*swarm.Config)(nil).New(nopLogger{})
		Expect(err).To(MatchError("swarm config is nil"))
	})

	It("creates a Swarm with the default socket", func() {
		sw, err := (&swarm.Config{}).New(nopLogger{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sw).NotTo(BeNil())
	})

	It("creates a Swarm with a configured socket", func() {
		sw, err := (&swarm.Config{Socket: "/tmp/docker.sock"}).New(nopLogger{})
		Expect(err).NotTo(HaveOccurred())
		Expect(sw).NotTo(BeNil())
	})
})
