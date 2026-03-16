package swarm_test

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Events", func() {
	var (
		client *ClientMock
		sw     *swarm.Swarm
		ctx    context.Context
		events <-chan swarm.Event
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &ClientMock{}
		sw = swarm.New(client, nopLogger{})
	})

	Describe("streaming events", func() {
		var collected []swarm.Event

		BeforeEach(func() {
			testData := loadTestData("events-fail-pause.ndjson")

			client.StreamLinesFunc = func(ctx context.Context, path string) (<-chan []byte, error) {
				ch := make(chan []byte)
				go func() {
					defer close(ch)
					scanner := bufio.NewScanner(bytes.NewReader(testData))
					for scanner.Scan() {
						line := make([]byte, len(scanner.Bytes()))
						copy(line, scanner.Bytes())
						ch <- line
					}
				}()
				return ch, nil
			}
		})

		JustBeforeEach(func() {
			events, err = sw.Events(ctx)
			Expect(err).NotTo(HaveOccurred())

			collected = nil
			for evt := range events {
				collected = append(collected, evt)
			}
		})

		It("should filter out network events", func() {
			// Test data has 35 lines: service, container, and network events
			// Only service and container should pass through
			for _, evt := range collected {
				Expect(evt.Type).To(BeElementOf("service", "container"))
			}
		})

		It("should parse service events", func() {
			var serviceEvents []swarm.Event
			for _, evt := range collected {
				if evt.Type == "service" {
					serviceEvents = append(serviceEvents, evt)
				}
			}

			Expect(serviceEvents).To(HaveLen(3))
			Expect(serviceEvents[0].Service).To(Equal("tag"))

			// Check first service event has no update state
			var payload swarm.ServiceEvent
			Expect(json.Unmarshal(serviceEvents[0].Payload, &payload)).To(Succeed())
			Expect(payload.Action).To(Equal("update"))
			Expect(payload.UpdateState).To(BeEmpty())

			// Second has "updating"
			Expect(json.Unmarshal(serviceEvents[1].Payload, &payload)).To(Succeed())
			Expect(payload.UpdateState).To(Equal("updating"))

			// Third has "completed"
			Expect(json.Unmarshal(serviceEvents[2].Payload, &payload)).To(Succeed())
			Expect(payload.UpdateState).To(Equal("completed"))
		})

		It("should parse container events", func() {
			var containerEvents []swarm.Event
			for _, evt := range collected {
				if evt.Type == "container" {
					containerEvents = append(containerEvents, evt)
				}
			}

			// Should have create, start, destroy, die events
			Expect(len(containerEvents)).To(BeNumerically(">", 0))

			// Check a container event has expected fields
			var payload swarm.ContainerEvent
			Expect(json.Unmarshal(containerEvents[0].Payload, &payload)).To(Succeed())
			Expect(payload.Action).To(Equal("create"))
			Expect(payload.TaskID).NotTo(BeEmpty())
			Expect(payload.TaskName).To(HavePrefix("tag.1."))
			Expect(payload.Image).To(Equal("local/tag:c197117"))
		})

		It("should capture exit code on die events", func() {
			var dieEvents []swarm.ContainerEvent
			for _, evt := range collected {
				if evt.Type == "container" {
					var payload swarm.ContainerEvent
					Expect(json.Unmarshal(evt.Payload, &payload)).To(Succeed())
					if payload.Action == "die" {
						dieEvents = append(dieEvents, payload)
					}
				}
			}

			Expect(len(dieEvents)).To(BeNumerically(">", 0))
			Expect(dieEvents[0].ExitCode).To(Equal("1"))
			Expect(dieEvents[0].ExecDur).To(Equal("30"))
		})

		It("should set timestamp from unix time", func() {
			Expect(collected[0].Time.Unix()).To(Equal(int64(1773515160)))
		})
	})
})
