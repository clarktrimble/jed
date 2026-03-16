package swarm_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

// nopLogger is a no-op logger for tests.
type nopLogger struct{}

func (nopLogger) Info(ctx context.Context, msg string, kv ...any)             {}
func (nopLogger) Debug(ctx context.Context, msg string, kv ...any)            {}
func (nopLogger) Error(ctx context.Context, msg string, err error, kv ...any) {}

func loadTestData(name string) []byte {
	data, err := os.ReadFile("../test/data/swarm/" + name)
	Expect(err).NotTo(HaveOccurred())
	return data
}

var _ = Describe("Swarm Client", func() {
	var (
		client *ClientMock
		sw     *swarm.Swarm
		ctx    context.Context
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &ClientMock{}
		sw = swarm.New(client, nopLogger{})
	})

	Describe("ListServices", func() {
		var svcs []swarm.Service

		BeforeEach(func() {
			testData := loadTestData("get-services.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				return json.Unmarshal(testData, rcv)
			}
		})

		JustBeforeEach(func() {
			svcs, err = sw.ListServices(ctx)
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should parse services", func() {
			Expect(svcs).To(HaveLen(4))
			Expect(svcs[0].ID).To(Equal("dhgtqt27zv3zf6n13g2zgp3b0"))
			Expect(svcs[0].Name).To(Equal("whoami"))
			Expect(svcs[3].Name).To(Equal("tag"))
		})
	})

	Describe("ListSecrets", func() {
		var secrets []swarm.Secret

		BeforeEach(func() {
			testData := loadTestData("get-secrets.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				return json.Unmarshal(testData, rcv)
			}
		})

		JustBeforeEach(func() {
			secrets, err = sw.ListSecrets(ctx)
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should parse secrets", func() {
			Expect(secrets).To(HaveLen(6))
			Expect(secrets[0].ID).To(Equal("53o7ekx6p3c5q22lzqgf3wv09"))
			Expect(secrets[0].Name).To(Equal("s3_secret_key_v1"))
		})
	})

	Describe("ListConfigs", func() {
		var configs []swarm.Config

		BeforeEach(func() {
			testData := loadTestData("get-configs.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				return json.Unmarshal(testData, rcv)
			}
		})

		JustBeforeEach(func() {
			configs, err = sw.ListConfigs(ctx)
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle empty list", func() {
			Expect(configs).To(HaveLen(0))
		})
	})

	Describe("ServiceTasks", func() {
		var tasks []swarm.Task

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

	Describe("CreateSecret", func() {
		var (
			id       string
			sentName string
			sentData string
		)

		BeforeEach(func() {
			testData := loadTestData("get-secrets.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				if method == "GET" && strings.Contains(path, "/secrets") {
					return json.Unmarshal(testData, rcv)
				}
				if method == "POST" && strings.Contains(path, "/secrets/create") {
					data, _ := json.Marshal(snd)
					var req map[string]string
					Expect(json.Unmarshal(data, &req)).To(Succeed())
					sentName = req["Name"]
					sentData = req["Data"]
					mockResponse(map[string]string{"ID": "new-secret-id"}, rcv)
				}
				return nil
			}
		})

		JustBeforeEach(func() {
			id, err = sw.CreateSecret(ctx, "bfc_api_key", []byte("supersecret"))
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the new ID", func() {
			Expect(id).To(Equal("new-secret-id"))
		})

		It("should use next version number", func() {
			Expect(sentName).To(Equal("bfc_api_key_v4"))
		})

		It("should base64 encode the data", func() {
			Expect(sentData).To(Equal("c3VwZXJzZWNyZXQ=")) // base64("supersecret")
		})
	})

	Describe("CreateConfig", func() {
		var (
			id       string
			sentName string
		)

		BeforeEach(func() {
			testData := loadTestData("get-configs.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				if method == "GET" && strings.Contains(path, "/configs") {
					return json.Unmarshal(testData, rcv)
				}
				if method == "POST" && strings.Contains(path, "/configs/create") {
					data, _ := json.Marshal(snd)
					var req map[string]string
					Expect(json.Unmarshal(data, &req)).To(Succeed())
					sentName = req["Name"]
					mockResponse(map[string]string{"ID": "new-config-id"}, rcv)
				}
				return nil
			}
		})

		JustBeforeEach(func() {
			id, err = sw.CreateConfig(ctx, "my_config", []byte("config data"))
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the new ID", func() {
			Expect(id).To(Equal("new-config-id"))
		})

		It("should start at v1 when no existing versions", func() {
			Expect(sentName).To(Equal("my_config_v1"))
		})
	})

	Describe("DeleteService", func() {
		var calledPath string

		BeforeEach(func() {
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				calledPath = path
				return nil
			}
		})

		JustBeforeEach(func() {
			err = sw.DeleteService(ctx, "my-service")
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call DELETE on correct path", func() {
			Expect(calledPath).To(Equal("/v1.52/services/my-service"))
		})
	})

	Describe("DeleteConfig", func() {
		var calledPath string

		BeforeEach(func() {
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				calledPath = path
				return nil
			}
		})

		JustBeforeEach(func() {
			err = sw.DeleteConfig(ctx, "config-id-123")
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call DELETE on correct path", func() {
			Expect(calledPath).To(Equal("/v1.52/configs/config-id-123"))
		})
	})

	Describe("CreateNetwork", func() {
		var (
			id          string
			sentRequest map[string]any
		)

		BeforeEach(func() {
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				data, _ := json.Marshal(snd)
				Expect(json.Unmarshal(data, &sentRequest)).To(Succeed())
				mockResponse(map[string]string{"ID": "net-123"}, rcv)
				return nil
			}
		})

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

	Describe("TaskLogs", func() {
		var logs []byte

		BeforeEach(func() {
			// Simulate multiplexed docker log format: 8-byte header + payload
			// Header: [stream_type, 0, 0, 0, size_byte1, size_byte2, size_byte3, size_byte4]
			// stdout = 1, stderr = 2
			rawLogs := []byte{
				1, 0, 0, 0, 0, 0, 0, 12, // stdout header, 12 bytes
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

	Describe("Status", func() {
		var (
			status  swarm.Status
			message string
		)

		// Todo: think about test data from swarm
		// Helper to build service response with given states
		serviceWith := func(desired, running int, updateState, updateMessage string) []map[string]any {
			return []map[string]any{{
				"ServiceStatus": map[string]any{
					"DesiredTasks": desired,
					"RunningTasks": running,
				},
				"UpdateStatus": map[string]any{
					"State":   updateState,
					"Message": updateMessage,
				},
			}}
		}

		Context("running service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(1, 1, "completed", ""), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, message, err = sw.Status(ctx, "myservice")
			})

			It("should return running", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusRunning))
				Expect(message).To(BeEmpty())
			})
		})

		Context("stopped service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(0, 0, "completed", ""), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, message, err = sw.Status(ctx, "myservice")
			})

			It("should return stopped", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusStopped))
			})
		})

		Context("stopping service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(0, 1, "completed", ""), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, message, err = sw.Status(ctx, "myservice")
			})

			It("should return pending", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusPending))
			})
		})

		Context("deploying service", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(1, 0, "updating", "update in progress"), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, message, err = sw.Status(ctx, "myservice")
			})

			It("should return pending with message", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusPending))
				Expect(message).To(Equal("update in progress"))
			})
		})

		Context("fresh service starting", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(1, 0, "", ""), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, message, err = sw.Status(ctx, "myservice")
			})

			It("should return pending", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusPending))
			})
		})

		Context("deploy failed (paused)", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(1, 0, "paused", "update paused due to failure"), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, message, err = sw.Status(ctx, "myservice")
			})

			It("should return error with message", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusError))
				Expect(message).To(Equal("update paused due to failure"))
			})
		})

		Context("task mismatch after completed update", func() {
			BeforeEach(func() {
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					mockResponse(serviceWith(2, 1, "completed", "update completed"), rcv)
					return nil
				}
			})

			JustBeforeEach(func() {
				status, message, err = sw.Status(ctx, "myservice")
			})

			It("should return error with message", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(status).To(Equal(swarm.StatusError))
				Expect(message).To(Equal("update completed"))
			})
		})
	})
})
