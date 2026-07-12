package swarm_test

import (
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Config", func() {
	It("creates a Swarm", func() {
		sw := (&swarm.Config{Socket: "/tmp/docker.sock"}).New(nopLogger{})
		Expect(sw).NotTo(BeNil())
	})
})
