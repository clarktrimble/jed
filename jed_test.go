package jed_test

import (
	"context"
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
)

func TestJed(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jed Suite")
}

var _ = Describe("Logs", func() {
	var (
		ctx     context.Context
		svc     *jed.Svc
		rawData []byte
		decoded []byte
		client  *ClientMock
		lgr     *LoggerMock
		err     error
	)

	BeforeEach(func() {
		ctx = context.Background()
		lgr = &LoggerMock{
			InfoFunc:  func(ctx context.Context, msg string, kv ...any) {},
			DebugFunc: func(ctx context.Context, msg string, kv ...any) {},
			ErrorFunc: func(ctx context.Context, msg string, err error, kv ...any) {},
		}
		client = &ClientMock{
			SendJsonFunc: func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				return rawData, nil
			},
		}
		cfg := &jed.Config{}
		svc = cfg.NewSvc(client, lgr)
	})

	JustBeforeEach(func() {
		decoded, err = svc.Logs(ctx, "test-container-id", "50")
	})

	When("given real Docker log data", func() {
		BeforeEach(func() {
			rawData, err = os.ReadFile("test/data/raw-log.bin")
			Expect(err).NotTo(HaveOccurred())
		})

		It("should decode without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call client with correct parameters", func() {
			Expect(client.SendJsonCalls()).To(HaveLen(1))
			call := client.SendJsonCalls()[0]
			Expect(call.Method).To(Equal("GET"))
			Expect(call.Path).To(Equal("/containers/test-container-id/logs?stdout=true&stderr=true&tail=50"))
			Expect(call.Body).To(BeNil())
		})

		It("should decode first and last lines correctly", func() {
			lines := strings.Split(string(decoded), "\n")
			Expect(lines).To(HaveLen(51)) // extra from trailing newline
			Expect(lines[0]).To(Equal(`{"app_id":"rsh","cmd":"datastore kafka-consumer-groups --group medic-dld-group --describe-offsets","level":"info","msg":"sending","request_id":"1lurhDR","run_id":"Zf1epR4","ts":"2025-10-14T22:09:26.02373568Z"}`))
			Expect(lines[49]).To(Equal(`{"app_id":"rsh","cmd":"exit","level":"info","msg":"sending","request_id":"1bxJ45g","run_id":"Zf1epR4","ts":"2025-10-14T22:10:26.011067266Z"}`))
		})
	})

	When("given empty data", func() {
		BeforeEach(func() {
			rawData = []byte{}
		})

		It("should return empty output without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(decoded).To(BeEmpty())
		})
	})
})
