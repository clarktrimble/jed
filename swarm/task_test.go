package swarm_test

import (
	"context"
	"encoding/json"
	"io"

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
		BeforeEach(func() {
			rawLogs := []byte{
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
	})
})
