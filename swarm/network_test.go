package swarm_test

import (
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Network", func() {
	var (
		client      *ClientMock
		sw          *swarm.Swarm
		ctx         context.Context
		err         error
		id          string
		sentRequest map[string]any
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &ClientMock{}
		sw = swarm.New(client, nopLogger{})

		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			data, _ := json.Marshal(snd)
			Expect(json.Unmarshal(data, &sentRequest)).To(Succeed())
			mockResponse(map[string]string{"ID": "net-123"}, rcv)
			return nil
		}
	})

	Describe("CreateNetwork", func() {
		Describe("with encryption", func() {
			JustBeforeEach(func() {
				id, err = sw.CreateNetwork(ctx, "my-net", true, true)
			})

			It("should return without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return the ID", func() {
				Expect(id).To(Equal("net-123"))
			})

			It("should set overlay driver", func() {
				Expect(sentRequest["Driver"]).To(Equal("overlay"))
			})

			It("should set attachable", func() {
				Expect(sentRequest["Attachable"]).To(BeTrue())
			})

			It("should set encrypted option", func() {
				opts := sentRequest["Options"].(map[string]any)
				Expect(opts["encrypted"]).To(Equal("true"))
			})
		})
	})
})
