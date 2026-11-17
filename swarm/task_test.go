package swarm_test

import (
	"context"
	"encoding/binary"
	"encoding/json"
	"errors"
	"io"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Task", func() {
	var (
		client *ClientMock
		sw     *swarm.Swarm
		ctx    context.Context
		err    error
		tasks  []swarm.Task
		logs   []byte
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &ClientMock{}
		sw = swarm.New(client, nopLogger{})
	})

	Describe("ServiceTasks", func() {
		BeforeEach(func() {
			testData := loadTestData("get-tasks-traefik.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				return json.Unmarshal(testData, rcv)
			}
		})

		JustBeforeEach(func() {
			tasks, err = sw.ServiceTasks(ctx, "traefik")
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should parse tasks", func() {
			Expect(tasks).To(HaveLen(4))
			Expect(tasks[0].ID).To(Equal("chw715vucthx2m2cci32nnl7o"))
			Expect(tasks[0].State).To(Equal("shutdown"))
			Expect(tasks[0].Image).To(Equal("traefik:v3.6.9"))
			Expect(tasks[0].Timestamp).To(Equal("2026-02-27T22:52:41.571791925Z"))
		})
	})

	Describe("TaskLogs", func() {
		var rawLogs []byte

		BeforeEach(func() {
			rawLogs = []byte{
				1, 0, 0, 0, 0, 0, 0, 12,
				'h', 'e', 'l', 'l', 'o', ' ', 'w', 'o', 'r', 'l', 'd', '\n',
			}
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				return rawLogs, nil
			}
		})

		JustBeforeEach(func() {
			logs, err = sw.TaskLogs(ctx, "task-123", "100")
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should decode the logs", func() {
			Expect(string(logs)).To(Equal("hello world\n"))
		})

		It("requests the task's stdout and stderr with the requested tail", func() {
			calls := client.SendJsonCalls()
			Expect(calls).To(HaveLen(1))
			Expect(calls[0].Method).To(Equal("GET"))
			Expect(calls[0].Path).To(Equal("/v1.52/tasks/task-123/logs?stdout=true&stderr=true&tail=100"))
			Expect(calls[0].Body).To(BeNil())
		})

		When("given real Docker log data", func() {
			BeforeEach(func() {
				var readErr error
				rawLogs, readErr = os.ReadFile("../test/data/raw-log.bin")
				Expect(readErr).NotTo(HaveOccurred())
			})

			It("decodes all fixture lines", func() {
				Expect(err).NotTo(HaveOccurred())
				lines := strings.Split(string(logs), "\n")
				Expect(lines).To(HaveLen(51))
				Expect(lines[0]).To(ContainSubstring(`"app_id":"rsh"`))
			})
		})

		When("the log stream is empty", func() {
			BeforeEach(func() { rawLogs = nil })

			It("returns no logs without an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(logs).To(BeEmpty())
			})
		})

		When("stdout and stderr have multiple frames", func() {
			BeforeEach(func() {
				rawLogs = append(dockerLogFrame(1, strings.Repeat("a", 4096)), dockerLogFrame(2, "tail\n")...)
			})

			It("preserves buffered payload tails and frame order", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(string(logs)).To(Equal(strings.Repeat("a", 4096) + "tail\n"))
			})
		})

		When("the frame header is truncated", func() {
			BeforeEach(func() { rawLogs = []byte{1, 0, 0} })

			It("reports the task and header decoding failure", func() {
				Expect(err).To(MatchError(And(
					ContainSubstring("failed to decode logs for task \"task-123\""),
					ContainSubstring("failed to read docker log header"),
				)))
				Expect(errors.Is(err, io.ErrUnexpectedEOF)).To(BeTrue())
			})
		})

		When("the frame payload is truncated", func() {
			BeforeEach(func() {
				rawLogs = []byte{1, 0, 0, 0, 0, 0, 0, 4, 'a', 'b'}
			})

			It("reports the task and payload decoding failure", func() {
				Expect(err).To(MatchError(And(
					ContainSubstring("failed to decode logs for task \"task-123\""),
					ContainSubstring("failed to read docker log frame"),
				)))
				Expect(errors.Is(err, io.ErrUnexpectedEOF)).To(BeTrue())
			})
		})

		When("the frame is exactly at the size limit", func() {
			BeforeEach(func() { rawLogs = dockerLogFrame(1, strings.Repeat("b", 65536)) })

			It("decodes the entire payload", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(string(logs)).To(Equal(strings.Repeat("b", 65536)))
			})
		})

		When("the frame exceeds the size limit", func() {
			BeforeEach(func() { rawLogs = []byte{1, 0, 0, 0, 0, 1, 0, 1} })

			It("rejects the frame before reading its missing payload", func() {
				Expect(err).To(MatchError(And(
					ContainSubstring("failed to decode logs for task \"task-123\""),
					ContainSubstring("docker frame exceeds decoder buffer size: 65536"),
				)))
			})
		})

		When("Docker cannot return the logs", func() {
			var transportErr error

			BeforeEach(func() {
				transportErr = errors.New("docker unavailable")
				client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
					return nil, transportErr
				}
			})

			It("preserves the transport error with task retrieval context", func() {
				Expect(err).To(MatchError(And(
					ContainSubstring("failed to get logs for task \"task-123\""),
					ContainSubstring("docker unavailable"),
				)))
				Expect(errors.Is(err, transportErr)).To(BeTrue())
			})
		})
	})
})

func dockerLogFrame(stream byte, payload string) []byte {
	frame := make([]byte, 8, 8+len(payload))
	frame[0] = stream
	binary.BigEndian.PutUint32(frame[4:8], uint32(len(payload)))
	return append(frame, payload...)
}
