package swarm_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/swarm"
)

/*
  Inconsistencies with client_test.go:

  1. deployer vs sw naming
  2. mockResponse is defined here but used in both files
  3. GetService test should probably be in client_test.go or pulled into the main Describe
  4. loadTestData pattern not used - inline os.ReadFile instead
*/

func TestSwarm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Swarm Suite")
}

var _ = Describe("Deploy", func() {
	var (
		client   *ClientMock
		deployer *swarm.Swarm
		ctx      context.Context
		svc      jed.Service
		env      jed.Env
		err      error
		id       string
	)

	BeforeEach(func() {
		ctx = context.Background()

		svc = jed.Service{
			Name:    "reauth-acp",
			Image:   "local/reauth-acp:a1b2c3d",
			Ports:   map[string]string{"3031/tcp": "8012"},
			Volumes: map[string]string{"svc-data": "/data"},
			Network: "svc-net",
			Restart: "on-failure",
			Secrets: []string{"aruba_client_secret"},
		}

		env = jed.Env{
			Name: "reauth-acp",
			Vars: map[string]string{
				"RTH_CLIENT_BASEURI": "https://10.35.44.62",
				"RTH_SERVER_PORT":    "3031",
			},
		}
	})

	Describe("creating a new service", func() {
		BeforeEach(func() {
			client = &ClientMock{
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					switch {
					// GetService: return empty list (not found)
					case method == "GET" && strings.Contains(path, "/services?"):
						mockResponse([]map[string]any{}, rcv)
						return nil

					// ListSecrets: return versioned secrets
					case method == "GET" && strings.Contains(path, "/secrets"):
						mockResponse([]secretItem{
							{ID: "sec-abc-123", Spec: specName{Name: "aruba_client_secret_v2"}},
							{ID: "sec-abc-100", Spec: specName{Name: "aruba_client_secret_v1"}},
						}, rcv)
						return nil

					// CreateService: return ID
					case method == "POST" && strings.Contains(path, "/services/create"):
						mockResponse(map[string]string{"ID": "svc-new-456"}, rcv)
						return nil

					default:
						return nil
					}
				},
			}
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should create without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal("svc-new-456"))
		})

		It("should call CreateService with correct spec", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			Expect(spec["Name"]).To(Equal("reauth-acp"))

			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			Expect(containerSpec["Image"]).To(Equal("local/reauth-acp:a1b2c3d"))

			// Check secrets resolved
			secrets := containerSpec["Secrets"].([]map[string]any)
			Expect(secrets).To(HaveLen(1))
			Expect(secrets[0]["SecretID"]).To(Equal("sec-abc-123"))
			Expect(secrets[0]["SecretName"]).To(Equal("aruba_client_secret_v2"))

			// Check env
			envLines := containerSpec["Env"].([]string)
			Expect(envLines).To(ContainElement("RTH_CLIENT_BASEURI=https://10.35.44.62"))
			Expect(envLines).To(ContainElement("RTH_SERVER_PORT=3031"))

			// Check mounts
			mounts := containerSpec["Mounts"].([]map[string]string)
			Expect(mounts).To(HaveLen(1))
			Expect(mounts[0]["Source"]).To(Equal("svc-data"))
			Expect(mounts[0]["Target"]).To(Equal("/data"))

			// Check ports
			endpointSpec := spec["EndpointSpec"].(map[string]any)
			ports := endpointSpec["Ports"].([]map[string]any)
			Expect(ports).To(HaveLen(1))
			Expect(ports[0]["TargetPort"]).To(Equal(3031))
			Expect(ports[0]["PublishedPort"]).To(Equal(8012))
			Expect(ports[0]["Protocol"]).To(Equal("tcp"))

			// Check network
			networks := taskTemplate["Networks"].([]map[string]string)
			Expect(networks).To(HaveLen(1))
			Expect(networks[0]["Target"]).To(Equal("svc-net"))
		})
	})

	Describe("updating an existing service", func() {
		BeforeEach(func() {
			client = &ClientMock{
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					switch {
					// GetService: return list with one service
					case method == "GET" && strings.Contains(path, "/services?"):
						mockResponse([]map[string]any{
							{"Version": map[string]any{"Index": 42}},
						}, rcv)
						return nil

					// ListSecrets
					case method == "GET" && strings.Contains(path, "/secrets"):
						mockResponse([]secretItem{
							{ID: "sec-abc-123", Spec: specName{Name: "aruba_client_secret_v2"}},
						}, rcv)
						return nil

					// UpdateService
					case method == "POST" && strings.Contains(path, "/update"):
						return nil

					default:
						return nil
					}
				},
			}
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should update without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(BeEmpty())
		})

		It("should call UpdateService with version", func() {
			calls := client.SendObjectCalls()
			updateCall := findCall(calls, "POST", "/update")
			Expect(updateCall).NotTo(BeNil())
			Expect(updateCall.Path).To(ContainSubstring("version=42"))
		})
	})

	Describe("with no secrets", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			client = newCreateMock("svc-new-789")
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should deploy without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should not include Secrets in spec", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			Expect(containerSpec).NotTo(HaveKey("Secrets"))
		})
	})

	Describe("with hosts", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Hosts = []string{"192.168.88.75 vilnius", "10.0.0.1 gateway"}
			client = newCreateMock("svc-hosts-123")
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should deploy without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include Hosts in container spec", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)

			hosts := containerSpec["Hosts"].([]string)
			Expect(hosts).To(HaveLen(2))
			Expect(hosts).To(ContainElement("192.168.88.75 vilnius"))
			Expect(hosts).To(ContainElement("10.0.0.1 gateway"))
		})
	})

	Describe("with publish_mode host", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.PublishMode = "host"
			client = newCreateMock("svc-host-mode")
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should deploy without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include PublishMode in port spec", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			endpointSpec := spec["EndpointSpec"].(map[string]any)
			ports := endpointSpec["Ports"].([]map[string]any)
			Expect(ports).To(HaveLen(1))
			Expect(ports[0]["PublishMode"]).To(Equal("host"))
		})
	})

	Describe("with custom user", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.User = "1000:967"
			client = newCreateMock("svc-custom-user")
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should deploy without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should use custom user in container spec", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			Expect(containerSpec["User"]).To(Equal("1000:967"))
		})
	})

	Describe("with default user", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			client = newCreateMock("svc-default-user")
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should use default user 1001", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			Expect(containerSpec["User"]).To(Equal("1001"))
		})
	})

	Describe("with traefik config", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080"}
			client = newCreateMock("svc-traefik")
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should deploy without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should generate traefik labels", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			labels := spec["Labels"].(map[string]string)

			Expect(labels["traefik.enable"]).To(Equal("true"))
			Expect(labels["traefik.http.routers.reauth-acp.rule"]).To(Equal("PathPrefix(`/reauth-acp`)"))
			Expect(labels["traefik.http.routers.reauth-acp.entrypoints"]).To(Equal("websecure"))
			Expect(labels["traefik.http.routers.reauth-acp.tls"]).To(Equal("true"))
			Expect(labels["traefik.http.routers.reauth-acp.middlewares"]).To(Equal("reauth-acp-strip"))
			Expect(labels["traefik.http.middlewares.reauth-acp-strip.stripprefix.prefixes"]).To(Equal("/reauth-acp"))
			Expect(labels["traefik.http.services.reauth-acp.loadbalancer.server.port"]).To(Equal("8080"))
		})
	})

	Describe("with traefik config and explicit labels", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080"}
			svc.Labels = map[string]string{
				"custom.label":   "custom-value",
				"traefik.enable": "false", // explicit override
			}
			client = newCreateMock("svc-traefik-explicit")
			deployer = swarm.New(client)
		})

		JustBeforeEach(func() {
			id, err = deployer.Deploy(ctx, svc, env)
		})

		It("should merge labels with explicit winning", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			labels := spec["Labels"].(map[string]string)

			// Custom label preserved
			Expect(labels["custom.label"]).To(Equal("custom-value"))
			// Explicit override wins
			Expect(labels["traefik.enable"]).To(Equal("false"))
			// Generated labels still present
			Expect(labels["traefik.http.routers.reauth-acp.rule"]).To(Equal("PathPrefix(`/reauth-acp`)"))
		})
	})
})

// helpers

type secretItem struct {
	ID   string   `json:"ID"`
	Spec specName `json:"Spec"`
}

type specName struct {
	Name string `json:"Name"`
}

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

//func fmt404Error() error {
//return fmt.Errorf("request failed: 404")
//}

func newCreateMock(id string) *ClientMock {
	return &ClientMock{
		SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
			switch {
			case method == "GET" && strings.Contains(path, "/services?"):
				mockResponse([]map[string]any{}, rcv) // empty list = not found
				return nil
			case method == "POST" && strings.Contains(path, "/services/create"):
				mockResponse(map[string]string{"ID": id}, rcv)
				return nil
			default:
				return nil
			}
		},
	}
}

var _ = Describe("GetService", func() {
	var (
		client   *ClientMock
		deployer *swarm.Swarm
		ctx      context.Context
		svcInfo  *swarm.ServiceInfo
		err      error
	)

	BeforeEach(func() {
		ctx = context.Background()

		testData, err := os.ReadFile("../test/data/swarm/get-services-filtered.json")
		Expect(err).NotTo(HaveOccurred())

		client = &ClientMock{
			SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
				err := json.Unmarshal(testData, rcv)
				Expect(err).NotTo(HaveOccurred())
				return nil
			},
		}
		deployer = swarm.New(client)
	})

	JustBeforeEach(func() {
		svcInfo, err = deployer.GetService(ctx, "tag")
	})

	It("should return without error", func() {
		Expect(err).NotTo(HaveOccurred())
	})

	It("should parse Version", func() {
		Expect(svcInfo.Version.Index).To(BeNumerically(">", 0))
	})

	It("should parse Spec as raw JSON", func() {
		Expect(svcInfo.Spec).NotTo(BeEmpty())
		Expect(string(svcInfo.Spec)).To(ContainSubstring("tag"))
	})

	It("should parse Endpoint", func() {
		Expect(svcInfo.Endpoint.Ports).To(HaveLen(1))
		Expect(svcInfo.Endpoint.Ports[0].TargetPort).To(Equal(3031))
		Expect(svcInfo.Endpoint.Ports[0].PublishedPort).To(Equal(8010))
		Expect(svcInfo.Endpoint.VirtualIPs).To(HaveLen(2))
	})

	It("should parse UpdateStatus", func() {
		Expect(svcInfo.UpdateStatus.State).To(Equal("completed"))
		Expect(svcInfo.UpdateStatus.CompletedAt).NotTo(BeZero())
	})

	It("should parse timestamps", func() {
		Expect(svcInfo.CreatedAt).NotTo(BeZero())
		Expect(svcInfo.UpdatedAt).NotTo(BeZero())
		Expect(svcInfo.UpdatedAt.After(svcInfo.CreatedAt)).To(BeTrue())
	})
})
