package container_test

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/container"
)

func TestContainer(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Container Suite")
}

var _ = Describe("Runtime", func() {
	var (
		client *ClientMock
		lgr    *LoggerMock
		rt     *container.Runtime
		ctx    context.Context
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()
		lgr = &LoggerMock{
			InfoFunc:  func(ctx context.Context, msg string, kv ...any) {},
			DebugFunc: func(ctx context.Context, msg string, kv ...any) {},
			ErrorFunc: func(ctx context.Context, msg string, err error, kv ...any) {},
		}
		client = &ClientMock{
			SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error { return nil },
			SendJsonFunc:   func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) { return nil, nil },
		}
		rt = container.New(client, lgr)
	})

	Describe("Deploy", func() {
		var id string
		svc := jed.Service{
			Name:    "test-app",
			Image:   "test:v1",
			Network: "test-net",
			Restart: jed.RestartPolicy{Condition: jed.RestartAny},
			Ports:   map[string]string{"8080/tcp": "8080"},
			Labels:  map[string]string{"app": "test", "version": "1.0"},
			Volumes: map[string]string{"/host/path": "/container/path"},
		}
		env := jed.Env{Name: "test-app", Vars: map[string]string{"PORT": "8080"}}

		JustBeforeEach(func() {
			id, err = rt.Deploy(ctx, svc, env)
		})

		When("deploying a valid container", func() {
			BeforeEach(func() {
				client.SendObjectFunc = mockCreate("abc123def456")
			})

			It("returns the container ID", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal("abc123def456"))
			})

			It("creates and starts the container", func() {
				calls := client.SendObjectCalls()
				Expect(calls).To(HaveLen(3)) // list, create, start

				createCall := findCall(calls, "POST", "/containers/create")
				startCall := findCall(calls, "POST", "/start")

				Expect(createCall).NotTo(BeNil())
				Expect(createCall.Path).To(MatchRegexp(`^/containers/create\?name=test-app-[a-zA-Z0-9]{7}$`))
				Expect(startCall).NotTo(BeNil())
				Expect(startCall.Path).To(MatchRegexp(`^/containers/test-app-[a-zA-Z0-9]{7}/start$`))
			})

			It("includes service configuration in create spec", func() {
				createCall := findCall(client.SendObjectCalls(), "POST", "/containers/create")
				Expect(createCall).NotTo(BeNil())

				var cfg map[string]any
				data, err := json.Marshal(createCall.Snd)
				Expect(err).NotTo(HaveOccurred())
				Expect(json.Unmarshal(data, &cfg)).To(Succeed())
				Expect(cfg["Image"]).To(Equal("test:v1"))
				Expect(cfg["Env"]).To(ContainElement("PORT=8080"))
				Expect(cfg["Labels"]).To(HaveKeyWithValue("managed_by", "jed"))

				hostCfg := cfg["HostConfig"].(map[string]any)
				Expect(hostCfg["Binds"]).To(ContainElement("/host/path:/container/path"))
				Expect(hostCfg["RestartPolicy"]).To(HaveKeyWithValue("Name", "always"))
			})
		})

		When("service is already deployed", func() {
			BeforeEach(func() {
				client.SendObjectFunc = mockContainers([]container.Container{{
					Id:     "existing123",
					Names:  []string{"/test-app-x7y9z2n"},
					Image:  "test:v1",
					State:  "running",
					Status: "Up 1 hour",
					Labels: map[string]string{"managed_by": "jed"},
				}})
			})

			It("returns an already deployed error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("test-app already deployed"))
			})
		})
	})

	Describe("Containers", func() {
		var containers container.Containers

		JustBeforeEach(func() {
			containers, err = rt.Containers(ctx)
		})

		When("multiple containers exist", func() {
			BeforeEach(func() {
				client.SendObjectFunc = mockContainers(testContainerList())
			})

			It("parses service names from deploy names", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(HaveLen(3))
				Expect(containers[0].ServiceName).To(Equal("postgres"))
			})
		})

		When("container name does not match suffix pattern", func() {
			BeforeEach(func() {
				client.SendObjectFunc = mockContainers([]container.Container{{
					Id:     "abc123",
					Names:  []string{"/postgres"},
					Image:  "postgres:14",
					State:  "running",
					Status: "Up 2 hours",
					Labels: map[string]string{"managed_by": "jed"},
				}})
			})

			It("skips it and logs", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(containers).To(BeEmpty())
				Expect(lgr.ErrorCalls()).To(HaveLen(1))
				Expect(lgr.ErrorCalls()[0].Msg).To(Equal("ignoring managed_by=jed containers"))
			})
		})
	})

	Describe("Undeploy", func() {
		JustBeforeEach(func() {
			err = rt.Undeploy(ctx, "test-app")
		})

		When("container exists", func() {
			BeforeEach(func() {
				cntrs := testContainerList()
				cntrs[0].Names = []string{"/test-app-x7y9z2n"}
				client.SendObjectFunc = mockContainers(cntrs[:1])
			})

			It("stops and deletes it", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(findCall(client.SendObjectCalls(), "POST", "/stop").Path).To(Equal("/containers/test-app-x7y9z2n/stop"))
				Expect(findCall(client.SendObjectCalls(), "DELETE", "/containers/").Path).To(Equal("/containers/test-app-x7y9z2n"))
			})
		})

		When("stop fails but delete succeeds", func() {
			BeforeEach(func() {
				cntrs := testContainerList()
				cntrs[0].Names = []string{"/test-app-x7y9z2n"}
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					if method == "GET" && strings.Contains(path, "/containers/json") {
						mockResponse(cntrs[:1], rcv)
						return nil
					}
					if method == "POST" && strings.Contains(path, "/stop") {
						return io.ErrUnexpectedEOF
					}
					return nil
				}
			})

			It("logs the stop error and succeeds", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(lgr.ErrorCalls()).To(HaveLen(1))
				Expect(lgr.ErrorCalls()[0].Err).To(Equal(io.ErrUnexpectedEOF))
			})
		})
	})

	Describe("Logs", func() {
		var decoded []byte
		var rawData []byte

		BeforeEach(func() {
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				return rawData, nil
			}
		})

		JustBeforeEach(func() {
			decoded, err = rt.Logs(ctx, "test-container-id", "50")
		})

		When("given real Docker log data", func() {
			BeforeEach(func() {
				var readErr error
				rawData, readErr = os.ReadFile("../test/data/raw-log.bin")
				Expect(readErr).NotTo(HaveOccurred())
			})

			It("decodes logs", func() {
				Expect(err).NotTo(HaveOccurred())
				lines := strings.Split(string(decoded), "\n")
				Expect(lines).To(HaveLen(51))
				Expect(lines[0]).To(ContainSubstring(`"app_id":"rsh"`))
			})
		})
	})
})

func mockResponse(obj, rcv any) {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	err = json.Unmarshal(data, rcv)
	Expect(err).NotTo(HaveOccurred())
}

func findCall(calls []struct {
	Ctx    context.Context
	Method string
	Path   string
	Snd    any
	Rcv    any
}, method, pathSubstring string) *struct {
	Ctx    context.Context
	Method string
	Path   string
	Snd    any
	Rcv    any
} {
	for i := range calls {
		if calls[i].Method == method && strings.Contains(calls[i].Path, pathSubstring) {
			return &calls[i]
		}
	}
	return nil
}

func mockContainers(containers []container.Container) func(context.Context, string, string, any, any) error {
	return func(ctx context.Context, method, path string, snd, rcv any) error {
		if method == "GET" && strings.Contains(path, "/containers/json") {
			mockResponse(containers, rcv)
		}
		return nil
	}
}

func testContainerList() []container.Container {
	return []container.Container{
		{Id: "abc123", Names: []string{"/postgres-x7y9z2n"}, Image: "postgres:14", State: "running", Status: "Up 2 hours", Labels: map[string]string{"managed_by": "jed"}},
		{Id: "def456", Names: []string{"/redis-k3m5p1q"}, Image: "redis:7", State: "running", Status: "Up 1 hour", Labels: map[string]string{"managed_by": "jed"}},
		{Id: "ghi789", Names: []string{"/nginx-w8x2y4z"}, Image: "nginx:latest", State: "exited", Status: "Exited (0) 5 minutes ago", Labels: map[string]string{"managed_by": "jed"}},
	}
}

func mockCreate(containerID string) func(context.Context, string, string, any, any) error {
	return func(ctx context.Context, method, path string, snd, rcv any) error {
		if method == "GET" && strings.Contains(path, "/containers/json") {
			mockResponse([]container.Container{}, rcv)
		}
		if method == "POST" && strings.Contains(path, "/containers/create") && rcv != nil {
			mockResponse(map[string]string{"Id": containerID}, rcv)
		}
		return nil
	}
}
