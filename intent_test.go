package jed_test

import (
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
)

var _ = Describe("Intent", func() {
	Describe("JSON", func() {
		It("uses lower-case field names", func() {
			data, err := json.Marshal(jed.Intent{Name: "app", Image: "app:v1", Replicas: 2})
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(ContainSubstring(`"name":"app"`))
			Expect(string(data)).To(ContainSubstring(`"image":"app:v1"`))
			Expect(string(data)).To(ContainSubstring(`"replicas":2`))
			Expect(string(data)).NotTo(ContainSubstring(`"Name"`))
			Expect(string(data)).NotTo(ContainSubstring(`"Image"`))
			Expect(string(data)).NotTo(ContainSubstring(`"Replicas"`))
		})
	})

	Describe("Validate", func() {
		It("accepts a minimal intent", func() {
			intent := jed.Intent{Name: "app", Image: "app:v1"}
			Expect(intent.Validate()).To(Succeed())
		})

		It("reports missing required fields", func() {
			intent := jed.Intent{}
			err := intent.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("name is required"))
			Expect(err.Error()).To(ContainSubstring("image is required"))
		})

		It("rejects negative replica counts", func() {
			intent := jed.Intent{Name: "app", Image: "app:v1", Replicas: -1}
			err := intent.Validate()
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("replicas cannot be negative"))
		})
	})
})
