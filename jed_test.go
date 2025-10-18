package jed_test

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
)

func TestJed(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Jed Suite")
}

// Test helpers

// mockResponse marshals obj to JSON and unmarshals into rcv.
func mockResponse(obj, rcv any) {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	err = json.Unmarshal(data, rcv)
	Expect(err).NotTo(HaveOccurred())
}

// findCall searches for a call matching method and path substring.
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

// mockContainers creates a mock response for Containers() call.
func mockContainers(containers []jed.Container) func(context.Context, string, string, any, any) error {
	return func(ctx context.Context, method, path string, snd, rcv any) error {
		if method == "GET" && strings.Contains(path, "/containers/json") {
			mockResponse(containers, rcv)
		}
		// Mock checkImage - always succeed
		if method == "GET" && strings.Contains(path, "/images/") {
			return nil
		}
		return nil
	}
}

// mockCreate creates a mock response for Deploy() create call.
func mockCreate(containerID string) func(context.Context, string, string, any, any) error {
	return func(ctx context.Context, method, path string, snd, rcv any) error {
		if method == "POST" && strings.Contains(path, "/containers/create") && rcv != nil {
			mockResponse(map[string]string{"Id": containerID}, rcv)
		}
		// Mock checkImage - always succeed
		if method == "GET" && strings.Contains(path, "/images/") {
			return nil
		}
		return nil
	}
}

var _ = Describe("Jed", func() {
	var (
		cfg    *jed.Config
		client *ClientMock
		lgr    *LoggerMock
		ctx    context.Context
		svc    *jed.Jed
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()
		cfg = &jed.Config{}
		lgr = &LoggerMock{
			InfoFunc:  func(ctx context.Context, msg string, kv ...any) {},
			DebugFunc: func(ctx context.Context, msg string, kv ...any) {},
			ErrorFunc: func(ctx context.Context, msg string, err error, kv ...any) {},
		}
		client = &ClientMock{
			// Default: mock checkImage to always succeed
			SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
				if method == "GET" && strings.Contains(path, "/images/") {
					return nil
				}
				return nil
			},
		}
	})

	Describe("NewSvc", func() {
		var (
			fsPath string
		)

		JustBeforeEach(func() {
			svc, err = cfg.NewJed(ctx, client, lgr, os.DirFS(fsPath))
		})

		When("given valid containers.yaml and env files", func() {
			BeforeEach(func() {
				fsPath = "test/data/cntr-cfg"
				// Mock checkImage calls for all images
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					if method == "GET" && strings.Contains(path, "/images/") {
						return nil // Image exists
					}
					return nil
				}
			})

			It("should create service without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(svc).NotTo(BeNil())
			})
		})

		When("given missing containers.yaml", func() {
			BeforeEach(func() {
				fsPath = "test/data/cntr-cfg-empty"
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
			})
		})

		When("given invalid container", func() {
			BeforeEach(func() {
				fsPath = "test/data/cntr-cfg-invalid"
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
			})
		})
	})

	Describe("Deploy", func() {
		var (
			cntr *jed.Service
			id   string
		)

		BeforeEach(func() {
			cntr = &jed.Service{
				Name:    "test-app",
				Image:   "test:v1",
				Network: "test-net",
				Restart: "always",
				Env:     map[string]string{"FOO": "bar", "BAZ": "qux"},
				Ports:   map[string]string{"8080/tcp": "8080"},
				Labels:  map[string]string{"app": "test", "version": "1.0"},
				Volumes: map[string]string{"/host/path": "/container/path"},
			}
			client.SendObjectFunc = mockCreate("abc123def456")
			svc, err = cfg.NewJed(ctx, client, lgr, os.DirFS("test/data/cntr-cfg"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			id, err = svc.Deploy(ctx, cntr)
		})

		When("deploying a valid container", func() {
			It("should return container ID without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(id).To(Equal("abc123def456"))
			})

			It("should call create and start", func() {
				calls := client.SendObjectCalls()
				// 4 checkImage calls (from NewSvc) + create + start
				Expect(calls).To(HaveLen(6))

				createCall := findCall(calls, "POST", "/containers/create")
				startCall := findCall(calls, "POST", "/start")

				Expect(createCall).NotTo(BeNil())
				Expect(createCall.Path).To(MatchRegexp(`^/containers/create\?name=test-app-[a-zA-Z0-9]{7}$`))
				Expect(startCall).NotTo(BeNil())
				Expect(startCall.Path).To(MatchRegexp(`^/containers/test-app-[a-zA-Z0-9]{7}/start$`))
			})
		})
	})

	Describe("Containers", func() {
		var (
			cntrs []jed.Service
		)

		BeforeEach(func() {
			svc, err = cfg.NewJed(ctx, client, lgr, os.DirFS("test/data/cntr-cfg"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			cntrs = svc.Services()
		})

		When("containers are loaded", func() {
			It("should return 4 containers", func() {
				Expect(cntrs).To(HaveLen(4))
			})

			It("should have correct names", func() {
				names := []string{}
				for _, cntr := range cntrs {
					names = append(names, cntr.Name)
				}
				Expect(names).To(ContainElements("traefik", "axis-camera-1", "logscale-1", "borken-1"))
			})

			It("should load container properties from YAML", func() {
				traefik := cntrs[0] // traefik is first in containers.yaml
				Expect(traefik.Name).To(Equal("traefik"))
				Expect(traefik.Image).To(Equal("traefik:v3.0"))
				Expect(traefik.Network).To(Equal("admin-int"))
				Expect(traefik.Restart).To(Equal("always"))
				Expect(traefik.Ports["80/tcp"]).To(Equal("80"))
				Expect(traefik.Ports["443/tcp"]).To(Equal("443"))
				Expect(traefik.Ports["8080/tcp"]).To(Equal("8080"))
				Expect(traefik.Volumes["/var/run/docker.sock"]).To(Equal("/var/run/docker.sock"))
				Expect(traefik.Volumes["/home/debian/proj/admin-int/certs"]).To(Equal("/certs"))
			})

			It("should load existing labels from YAML", func() {
				axisCamera := cntrs[1] // axis-camera-1 is second in containers.yaml
				Expect(axisCamera.Name).To(Equal("axis-camera-1"))
				Expect(axisCamera.Labels["traefik.enable"]).To(Equal("true"))
				Expect(axisCamera.Labels["traefik.http.routers.axis-camera-1.tls"]).To(Equal("true"))
				Expect(axisCamera.Labels["traefik.http.routers.axis-camera-1.rule"]).To(Equal("Host(`axis.int.bastille.cloud`)"))
			})

			It("should load env files for containers that have them", func() {
				traefik := cntrs[0] // traefik is first in containers.yaml
				Expect(traefik.Name).To(Equal("traefik"))
				Expect(traefik.Env["TRAEFIK_API_DASHBOARD"]).To(Equal("true"))
				Expect(traefik.Env["TRAEFIK_PROVIDERS_DOCKER"]).To(Equal("true"))
				Expect(traefik.Env["TRAEFIK_ENTRYPOINTS_WEB_ADDRESS"]).To(Equal(":80"))
			})

			It("should handle containers without env files", func() {
				borken := cntrs[3] // borken-1 has no .env file
				Expect(borken.Name).To(Equal("borken-1"))
				Expect(borken.Env).To(BeEmpty())
			})
		})
	})

	Describe("Undeploy", func() {
		var (
			cntr *jed.Service
		)

		BeforeEach(func() {
			cntr = &jed.Service{
				Name: "test-app",
			}
			svc, err = cfg.NewJed(ctx, client, lgr, os.DirFS("test/data/cntr-cfg"))
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			err = svc.Undeploy(ctx, cntr)
		})

		When("container exists and stops successfully", func() {
			BeforeEach(func() {
				statuses := []jed.Container{
					{
						Id:     "abc123",
						Names:  []string{"/test-app-x7y9z2n"},
						State:  "running",
						Labels: map[string]string{"managed_by": "jed"},
					},
				}
				client.SendObjectFunc = mockContainers(statuses)
			})

			It("should undeploy without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should call stop and delete", func() {
				calls := client.SendObjectCalls()
				// 4 checkImage (from NewSvc) + Containers + stop + delete
				Expect(calls).To(HaveLen(7))

				stopCall := findCall(calls, "POST", "/stop")
				deleteCall := findCall(calls, "DELETE", "/containers/")

				Expect(stopCall).NotTo(BeNil())
				Expect(stopCall.Path).To(Equal("/containers/test-app-x7y9z2n/stop"))
				Expect(deleteCall).NotTo(BeNil())
				Expect(deleteCall.Path).To(Equal("/containers/test-app-x7y9z2n"))
			})
		})

		When("stop fails but delete succeeds", func() {
			BeforeEach(func() {
				statuses := []jed.Container{
					{
						Id:     "abc123",
						Names:  []string{"/test-app-x7y9z2n"},
						State:  "created",
						Labels: map[string]string{"managed_by": "jed"},
					},
				}
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					// Mock Containers() call
					if method == "GET" && strings.Contains(path, "/containers/json") {
						mockResponse(statuses, rcv)
						return nil
					}
					// Mock stop() - fails
					if method == "POST" && strings.Contains(path, "/stop") {
						return io.ErrUnexpectedEOF
					}
					// Mock delete() - succeeds
					return nil
				}
			})

			It("should succeed (best effort)", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should log the stop error", func() {
				Expect(lgr.ErrorCalls()).To(HaveLen(1))
				call := lgr.ErrorCalls()[0]
				Expect(call.Msg).To(Equal("failed to stop container"))
				Expect(call.Err).To(Equal(io.ErrUnexpectedEOF))
			})
		})

		When("delete fails", func() {
			BeforeEach(func() {
				statuses := []jed.Container{
					{
						Id:     "abc123",
						Names:  []string{"/test-app-x7y9z2n"},
						State:  "running",
						Labels: map[string]string{"managed_by": "jed"},
					},
				}
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					// Mock Containers() call
					if method == "GET" && strings.Contains(path, "/containers/json") {
						mockResponse(statuses, rcv)
						return nil
					}
					// Mock delete() - fails
					if method == "DELETE" {
						return io.ErrClosedPipe
					}
					// Mock stop() - succeeds
					return nil
				}
			})

			It("should return delete error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err).To(Equal(io.ErrClosedPipe))
			})
		})

		When("container not found", func() {
			BeforeEach(func() {
				client.SendObjectFunc = mockContainers([]jed.Container{})
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("container test-app not found"))
			})
		})
	})

	Describe("Logs", func() {
		var (
			rawData []byte
			decoded []byte
		)

		BeforeEach(func() {
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				return rawData, nil
			}
			svc, err = cfg.NewJed(ctx, client, lgr, os.DirFS("test/data/cntr-cfg"))
			Expect(err).NotTo(HaveOccurred())
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
})
