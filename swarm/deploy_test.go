package swarm_test

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/swarm"
)

func TestSwarm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Swarm Suite")
}

var _ = Describe("Deploy", func() {
	var (
		client   *ClientMock
		deployer *swarm.Deployer
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
					// ServiceVersion: return 404
					case method == "GET" && strings.Contains(path, "/services/reauth-acp"):
						return fmt404Error()

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
					// ServiceVersion: return version 42
					case method == "GET" && strings.Contains(path, "/services/reauth-acp"):
						mockResponse(map[string]any{
							"Version": map[string]any{"Index": 42},
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

			client = &ClientMock{
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					switch {
					case method == "GET" && strings.Contains(path, "/services/reauth-acp"):
						return fmt404Error()

					case method == "POST" && strings.Contains(path, "/services/create"):
						mockResponse(map[string]string{"ID": "svc-new-789"}, rcv)
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

func fmt404Error() error {
	return fmt.Errorf("request failed: 404")
}
