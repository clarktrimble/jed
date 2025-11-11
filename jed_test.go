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

var _ = Describe("Jed", func() {
	var (
		cfg    *jed.Config
		client *ClientMock
		lgr    *LoggerMock
		ctx    context.Context
		svc    *jed.Jed
		err    error
		store  *StoreMock
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
		// Use mock store instead of bbolt for faster, isolated tests
		store = loadMockStoreFromFS("test/data/svc-cfg")
	})

	Describe("New", func() {

		JustBeforeEach(func() {
			svc, err = cfg.New(ctx, client, lgr, store)
		})

		When("given valid containers.yaml and env files", func() {

			It("should create service without error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(svc).NotTo(BeNil())
			})

			It("should add managed_by label to all services", func() {
				services, err := svc.Services(ctx)
				Expect(err).NotTo(HaveOccurred())
				Expect(services).NotTo(BeEmpty())
				for _, service := range services {
					Expect(service.Labels).To(HaveKeyWithValue("managed_by", "jed"))
				}
			})
		})
	})

	Describe("Deploy", func() {
		var (
			cntr jed.Service
			id   string
		)

		BeforeEach(func() {
			// Default valid service with all fields populated
			cntr = jed.Service{
				Name:    "test-app",
				Image:   "test:v1",
				Network: "test-net",
				Restart: "always",
				Ports:   map[string]string{"8080/tcp": "8080"},
				Labels:  map[string]string{"app": "test", "version": "1.0"},
				Volumes: map[string]string{"/host/path": "/container/path"},
			}
			client.SendObjectFunc = mockCreate("abc123def456")
			svc, err = cfg.New(ctx, client, lgr, store)
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
				// 4 checkImage calls (from NewSvc) + containers + create + start
				// Todo: improve on len/find checking??
				Expect(calls).To(HaveLen(7))

				createCall := findCall(calls, "POST", "/containers/create")
				startCall := findCall(calls, "POST", "/start")

				Expect(createCall).NotTo(BeNil())
				Expect(createCall.Path).To(MatchRegexp(`^/containers/create\?name=test-app-[a-zA-Z0-9]{7}$`))
				Expect(startCall).NotTo(BeNil())
				Expect(startCall.Path).To(MatchRegexp(`^/containers/test-app-[a-zA-Z0-9]{7}/start$`))
			})
		})
	})

	Describe("Services", func() {
		var (
			services jed.Services
		)

		BeforeEach(func() {
			svc, err = cfg.New(ctx, client, lgr, store)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			services, err = svc.Services(ctx)
			Expect(err).NotTo(HaveOccurred())
		})

		When("services are loaded", func() {
			It("should return 4 services", func() {
				Expect(services).To(HaveLen(4))
			})

			It("should have correct names", func() {
				_, err := services.Find("traefik")
				Expect(err).NotTo(HaveOccurred())
				_, err = services.Find("axis-camera-1")
				Expect(err).NotTo(HaveOccurred())
				_, err = services.Find("logscale-1")
				Expect(err).NotTo(HaveOccurred())
				_, err = services.Find("borken-1")
				Expect(err).NotTo(HaveOccurred())
			})

			It("should load service properties from YAML", func() {
				traefik, err := services.Find("traefik")
				Expect(err).NotTo(HaveOccurred())
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
				axisCamera, err := services.Find("axis-camera-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(axisCamera.Name).To(Equal("axis-camera-1"))
				Expect(axisCamera.Labels["traefik.enable"]).To(Equal("true"))
				Expect(axisCamera.Labels["traefik.http.routers.axis-camera-1.tls"]).To(Equal("true"))
				Expect(axisCamera.Labels["traefik.http.routers.axis-camera-1.rule"]).To(Equal("Host(`axis.int.bastille.cloud`)"))
			})
		})
	})

	Describe("Undeploy", func() {
		var (
			cntr jed.Service
		)

		BeforeEach(func() {
			cntr = jed.Service{
				Name: "test-app",
			}
			svc, err = cfg.New(ctx, client, lgr, store)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			err = svc.Undeploy(ctx, cntr)
		})

		When("container exists and stops successfully", func() {
			BeforeEach(func() {
				cntrs := testContainerList()
				cntrs[0].Names = []string{"/test-app-x7y9z2n"}
				client.SendObjectFunc = mockContainers(cntrs[:1])
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
				cntrs := testContainerList()
				cntrs[0].Names = []string{"/test-app-x7y9z2n"}
				cntrs[0].State = "created"
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					// Mock Containers() call
					if method == "GET" && strings.Contains(path, "/containers/json") {
						mockResponse(cntrs[:1], rcv)
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
				cntrs := testContainerList()
				cntrs[0].Names = []string{"/test-app-x7y9z2n"}
				client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
					// Mock Containers() call
					if method == "GET" && strings.Contains(path, "/containers/json") {
						mockResponse(cntrs[:1], rcv)
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
			svc, err = cfg.New(ctx, client, lgr, store)
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

	Describe("CreateService", func() {
		var (
			newService jed.Service
		)

		BeforeEach(func() {
			newService = jed.Service{
				Name:    "new-service",
				Image:   "nginx:latest",
				Network: "test-net",
				Restart: "always",
			}
			svc, err = cfg.New(ctx, client, lgr, store)
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			err = svc.CreateService(ctx, newService)
		})

		When("creating a valid service", func() {
			It("should create without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should add service to Services()", func() {
				services, err := svc.Services(ctx)
				Expect(err).NotTo(HaveOccurred())

				newSvc, err := services.Find("new-service")
				Expect(err).NotTo(HaveOccurred())
				Expect(newSvc.Image).To(Equal("nginx:latest"))
			})

			It("should persist service to store", func() {
				stored, err := store.GetServiceFunc(ctx, "new-service")
				Expect(err).NotTo(HaveOccurred())
				Expect(stored.Name).To(Equal("new-service"))
				Expect(stored.Image).To(Equal("nginx:latest"))
			})
		})

		When("creating a duplicate service", func() {
			BeforeEach(func() {
				// Create the service first
				err := svc.CreateService(ctx, newService)
				Expect(err).NotTo(HaveOccurred())
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("already exists"))
			})
		})

		When("creating an invalid service", func() {
			BeforeEach(func() {
				newService = jed.Service{
					Name: "invalid",
					// Missing required fields
				}
			})

			It("should return validation error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("invalid"))
			})
		})
	})

	Describe("DeleteService", func() {
		var (
			serviceName string
		)

		BeforeEach(func() {
			svc, err = cfg.New(ctx, client, lgr, store)
			Expect(err).NotTo(HaveOccurred())

			serviceName = "traefik"
		})

		JustBeforeEach(func() {
			err = svc.DeleteService(ctx, serviceName)
		})

		When("deleting an existing service", func() {
			It("should delete without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should remove service from Services()", func() {
				services, err := svc.Services(ctx)
				Expect(err).NotTo(HaveOccurred())

				_, err = services.Find("traefik")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("service traefik not found"))
			})

			It("should remove service from store", func() {
				_, err := store.GetServiceFunc(ctx, "traefik")
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})

			It("should remove env vars from store", func() {
				env, err := store.GetEnvFunc(ctx, "traefik")
				Expect(err).NotTo(HaveOccurred())
				Expect(env.Vars).To(BeEmpty())
			})
		})

		When("deleting a non-existent service", func() {
			BeforeEach(func() {
				serviceName = "nonexistent"
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})
	})

	Describe("SetEnv", func() {
		var (
			serviceName string
			env         map[string]string
		)

		BeforeEach(func() {
			svc, err = cfg.New(ctx, client, lgr, store)
			Expect(err).NotTo(HaveOccurred())

			serviceName = "borken-1"
			env = map[string]string{
				"FOO": "bar",
				"BAZ": "qux",
			}
		})

		JustBeforeEach(func() {
			err = svc.SetEnv(ctx, serviceName, env)
		})

		When("setting env for existing service", func() {
			It("should set without error", func() {
				Expect(err).NotTo(HaveOccurred())
			})

			It("should persist env to store", func() {
				storedEnv, err := store.GetEnvFunc(ctx, "borken-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(storedEnv.Vars).To(HaveKeyWithValue("FOO", "bar"))
				Expect(storedEnv.Vars).To(HaveKeyWithValue("BAZ", "qux"))
			})

			It("should be retrievable via GetEnv", func() {
				retrieved, err := svc.GetEnv(ctx, "borken-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(retrieved).To(Equal(env))
			})
		})

		When("setting env for non-existent service", func() {
			BeforeEach(func() {
				serviceName = "nonexistent"
			})

			It("should return error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("not found"))
			})
		})
	})

	Describe("GetEnv", func() {

		BeforeEach(func() {
			svc, err = cfg.New(ctx, client, lgr, store)
			Expect(err).NotTo(HaveOccurred())
		})

		When("getting env for service with env", func() {
			It("should get without error", func() {
				retrieved, err := svc.GetEnv(ctx, "traefik")
				Expect(err).NotTo(HaveOccurred())
				Expect(retrieved).To(HaveKeyWithValue("TRAEFIK_API_DASHBOARD", "true"))
				Expect(retrieved).To(HaveKeyWithValue("TRAEFIK_PROVIDERS_DOCKER", "true"))
				Expect(retrieved).To(HaveKeyWithValue("TRAEFIK_ENTRYPOINTS_WEB_ADDRESS", ":80"))
			})
		})

		When("getting env for service without env", func() {
			It("should return empty map", func() {
				retrieved, err := svc.GetEnv(ctx, "borken-1")
				Expect(err).NotTo(HaveOccurred())
				Expect(retrieved).To(BeEmpty())
			})
		})
	})
})
