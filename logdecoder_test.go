package jed_test

import (
	"bytes"
	"context"
	"io"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
)

//go:generate moq -out mock_test.go -pkg jed_test . Logger  Client

func TestLogDecoder(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jed Suite")
}

var _ = Describe("decodeLogs", func() {
	var (
		rawData []byte
		decoded []byte
		lgr     *LoggerMock
		err     error
	)

	BeforeEach(func() {
		lgr = &LoggerMock{
			InfoFunc:  func(ctx context.Context, msg string, kv ...any) {},
			DebugFunc: func(ctx context.Context, msg string, kv ...any) {},
			ErrorFunc: func(ctx context.Context, msg string, err error, kv ...any) {},
		}
	})

	JustBeforeEach(func() {
		reader := jed.DecodeLogs(bytes.NewReader(rawData))
		decoded, err = io.ReadAll(reader)
	})

	When("given real Docker log data", func() {
		BeforeEach(func() {
			var readErr error
			rawData, readErr = os.ReadFile("test/data/raw-log.bin")
			Expect(readErr).NotTo(HaveOccurred())
		})

		It("should decode without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should produce decoded output", func() {
			Expect(decoded).NotTo(BeEmpty())
			Expect(len(decoded)).To(BeNumerically(">", 0))
		})

		It("should contain expected JSON log content", func() {
			output := string(decoded)
			Expect(output).To(ContainSubstring(`"app_id":"rsh"`))
			Expect(output).To(ContainSubstring(`"level":"info"`))
			Expect(output).To(ContainSubstring("kafka-consumer-groups"))
		})

		It("should be shorter than raw data (headers removed)", func() {
			// Each frame has 8 bytes of header that get stripped
			Expect(len(decoded)).To(BeNumerically("<", len(rawData)))
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
