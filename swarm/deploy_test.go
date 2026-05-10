package swarm_test

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Deploy", func() {
	var (
		client   *ClientMock
		deployer *swarm.Swarm
		ctx      context.Context
		svc      jed.Service
		env      jed.Env
		err      error
		id       string
		created  bool
	)

	BeforeEach(func() {
		ctx = context.Background()

		svc = jed.Service{
			Name:    "reauth-acp",
			Image:   "local/reauth-acp:a1b2c3d",
			Ports:   map[string]string{"3031/tcp": "8012"},
			Volumes: map[string]string{"svc-data": "/data"},
			Network: "svc-net",
			Restart: jed.RestartOnFailure,
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
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deploySpec(ctx, deployer, svc, env)
		})

		It("should create without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal("svc-new-456"))
			Expect(created).To(BeTrue())
		})

		It("should call CreateService with a swarm spec", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(swarm.Spec)
			Expect(spec["Name"]).To(Equal("reauth-acp"))
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
							{
								"ID":      "svc-existing-123",
								"Version": map[string]any{"Index": 42},
								"Spec": map[string]any{
									"TaskTemplate": map[string]any{"ForceUpdate": 0},
								},
							},
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
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deploySpec(ctx, deployer, svc, env)
		})

		It("should update without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal("svc-existing-123"))
			Expect(created).To(BeFalse())
		})

		It("should call UpdateService with version", func() {
			calls := client.SendObjectCalls()
			updateCall := findCall(calls, "POST", "/update")
			Expect(updateCall).NotTo(BeNil())
			Expect(updateCall.Path).To(ContainSubstring("version=42"))
		})
	})

	Describe("with invalid service", func() {
		BeforeEach(func() {
			svc.Network = ""
			client = &ClientMock{}
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deploySpec(ctx, deployer, svc, env)
		})

		It("should fail before calling Docker", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to validate service"))
			Expect(err.Error()).To(ContainSubstring("network is required"))
			Expect(id).To(BeEmpty())
			Expect(created).To(BeFalse())
			Expect(client.SendObjectCalls()).To(BeEmpty())
		})
	})

	Describe("when service lookup fails with another not found error", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			client = &ClientMock{
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					if method == "GET" && strings.Contains(path, "/services?") {
						return errors.New("backend index not found")
					}
					return nil
				},
			}
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deploySpec(ctx, deployer, svc, env)
		})

		It("should not treat it as a missing service", func() {
			Expect(err).To(HaveOccurred())
			Expect(id).To(BeEmpty())
			Expect(created).To(BeFalse())
			Expect(findCall(client.SendObjectCalls(), "POST", "/services/create")).To(BeNil())
		})
	})

	Describe("preserving force update on normal deploy", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			client = &ClientMock{
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					switch {
					case method == "GET" && strings.Contains(path, "/services?"):
						mockResponse([]map[string]any{{
							"ID":      "svc-existing-123",
							"Version": map[string]any{"Index": 42},
							"Spec": map[string]any{
								"TaskTemplate": map[string]any{"ForceUpdate": 7},
							},
						}}, rcv)
						return nil
					case method == "POST" && strings.Contains(path, "/update"):
						return nil
					default:
						return nil
					}
				},
			}
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deploySpec(ctx, deployer, svc, env)
		})

		It("keeps Docker's current ForceUpdate value", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(created).To(BeFalse())

			updateCall := findCall(client.SendObjectCalls(), "POST", "/update")
			Expect(updateCall).NotTo(BeNil())
			Expect(sentForceUpdate(updateCall.Snd)).To(Equal(7))
		})
	})

	Describe("Restart", func() {
		BeforeEach(func() {
			client = &ClientMock{
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					switch {
					case method == "GET" && strings.Contains(path, "/services?"):
						mockResponse([]map[string]any{{
							"ID":      "svc-existing-123",
							"Version": map[string]any{"Index": 42},
							"Spec": map[string]any{
								"Name": "reauth-acp",
								"TaskTemplate": map[string]any{
									"ForceUpdate": 7,
									"ContainerSpec": map[string]any{
										"Image": "local/reauth-acp:a1b2c3d",
									},
								},
							},
						}}, rcv)
						return nil
					case method == "POST" && strings.Contains(path, "/update"):
						return nil
					default:
						return nil
					}
				},
			}
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			err = deployer.Restart(ctx, svc.Name)
		})

		It("increments ForceUpdate and updates with the current version", func() {
			Expect(err).NotTo(HaveOccurred())

			updateCall := findCall(client.SendObjectCalls(), "POST", "/update")
			Expect(updateCall).NotTo(BeNil())
			Expect(updateCall.Path).To(ContainSubstring("version=42"))

			Expect(sentForceUpdate(updateCall.Snd)).To(Equal(8))
		})
	})

	Describe("Restart with missing force update", func() {
		BeforeEach(func() {
			client = &ClientMock{
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					switch {
					case method == "GET" && strings.Contains(path, "/services?"):
						mockResponse([]map[string]any{{
							"ID":      "svc-existing-123",
							"Version": map[string]any{"Index": 42},
							"Spec": map[string]any{
								"Name":         "reauth-acp",
								"TaskTemplate": map[string]any{},
							},
						}}, rcv)
						return nil
					case method == "POST" && strings.Contains(path, "/update"):
						return nil
					default:
						return nil
					}
				},
			}
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			err = deployer.Restart(ctx, svc.Name)
		})

		It("treats missing ForceUpdate as zero", func() {
			Expect(err).NotTo(HaveOccurred())

			updateCall := findCall(client.SendObjectCalls(), "POST", "/update")
			Expect(updateCall).NotTo(BeNil())
			Expect(sentForceUpdate(updateCall.Snd)).To(Equal(1))
		})
	})

})

func deploySpec(ctx context.Context, deployer *swarm.Swarm, svc jed.Service, env jed.Env) (string, bool, error) {
	return deployer.Deploy(ctx, jed.Spec{Service: svc, Env: env})
}

func sentForceUpdate(snd any) int {
	data, err := json.Marshal(snd)
	Expect(err).NotTo(HaveOccurred())

	var spec struct {
		TaskTemplate struct {
			ForceUpdate int `json:"ForceUpdate"`
		} `json:"TaskTemplate"`
	}
	err = json.Unmarshal(data, &spec)
	Expect(err).NotTo(HaveOccurred())
	return spec.TaskTemplate.ForceUpdate
}
