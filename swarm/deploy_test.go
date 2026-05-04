package swarm_test

import (
	"context"
	"encoding/json"
	"errors"
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
			Restart: jed.RestartPolicy{Condition: jed.RestartOnFailure},
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
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should create without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(Equal("svc-new-456"))
			Expect(created).To(BeTrue())
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
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should update without error", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(id).To(BeEmpty())
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
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
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
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should not treat it as a missing service", func() {
			Expect(err).To(HaveOccurred())
			Expect(id).To(BeEmpty())
			Expect(created).To(BeFalse())
			Expect(findCall(client.SendObjectCalls(), "POST", "/services/create")).To(BeNil())
		})
	})

	Describe("with no secrets", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			client = newCreateMock("svc-new-789")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
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

	Describe("with default restart policy", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Restart = jed.RestartPolicy{}
			client = newCreateMock("svc-restart-default")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should disable restarts", func() {
			Expect(err).NotTo(HaveOccurred())

			createCall := findCall(client.SendObjectCalls(), "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			restart := taskTemplate["RestartPolicy"].(map[string]any)
			Expect(restart).To(Equal(map[string]any{"Condition": "none"}))
		})
	})

	Describe("with on-failure restart policy", func() {
		BeforeEach(func() {
			attempts := 3
			svc.Secrets = nil
			svc.Restart = jed.RestartPolicy{Condition: jed.RestartOnFailure, MaxAttempts: &attempts}
			client = newCreateMock("svc-restart-on-failure")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should include restart details", func() {
			Expect(err).NotTo(HaveOccurred())

			createCall := findCall(client.SendObjectCalls(), "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			restart := taskTemplate["RestartPolicy"].(map[string]any)
			Expect(restart["Condition"]).To(Equal("on-failure"))
			Expect(restart["Delay"]).To(Equal(5000000000))
			Expect(restart["MaxAttempts"]).To(Equal(3))
		})
	})

	Describe("with configs", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Configs = map[string]string{
				"reauth_config": "/etc/reauth/config.yaml",
			}
			client = &ClientMock{
				SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
					switch {
					case method == "GET" && strings.Contains(path, "/services?"):
						mockResponse([]map[string]any{}, rcv)
						return nil
					case method == "GET" && strings.Contains(path, "/configs"):
						mockResponse([]secretItem{
							{ID: "cfg-abc-123", Spec: specName{Name: "reauth_config_v3"}},
							{ID: "cfg-abc-100", Spec: specName{Name: "reauth_config_v1"}},
						}, rcv)
						return nil
					case method == "POST" && strings.Contains(path, "/services/create"):
						mockResponse(map[string]string{"ID": "svc-cfg-456"}, rcv)
						return nil
					default:
						return nil
					}
				},
			}
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should deploy without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should include resolved configs in spec", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)

			configs := containerSpec["Configs"].([]map[string]any)
			Expect(configs).To(HaveLen(1))
			Expect(configs[0]["ConfigID"]).To(Equal("cfg-abc-123"))
			Expect(configs[0]["ConfigName"]).To(Equal("reauth_config_v3"))

			file := configs[0]["File"].(map[string]any)
			Expect(file["Name"]).To(Equal("/etc/reauth/config.yaml"))
		})
	})

	Describe("with hosts", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Hosts = []string{"192.168.88.75 vilnius", "10.0.0.1 gateway"}
			client = newCreateMock("svc-hosts-123")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
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
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
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
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
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
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should use default user 1001", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			Expect(containerSpec["User"]).To(Equal("1001:1001"))
		})
	})

	Describe("with traefik config and strip", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080", PathPrefixStrip: true}
			client = newCreateMock("svc-traefik")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
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

	Describe("with traefik config no strip", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080"}
			client = newCreateMock("svc-traefik-nostrip")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should not include strip middleware", func() {
			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.Snd.(map[string]any)
			labels := spec["Labels"].(map[string]string)

			Expect(labels["traefik.enable"]).To(Equal("true"))
			Expect(labels["traefik.http.routers.reauth-acp.rule"]).To(Equal("PathPrefix(`/reauth-acp`)"))
			Expect(labels).NotTo(HaveKey("traefik.http.routers.reauth-acp.middlewares"))
			Expect(labels).NotTo(HaveKey("traefik.http.middlewares.reauth-acp-strip.stripprefix.prefixes"))
		})
	})

	Describe("with template vars in command", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Command = []string{
				"elasticsearch_exporter",
				"--es.uri={{ES_URI}}",
				"--es.ssl-skip-verify",
			}
			env.Vars["ES_URI"] = "https://elastic.example.com:9200"
			client = newCreateMock("svc-tpl-cmd")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should expand template vars in command", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			command := containerSpec["Command"].([]string)
			Expect(command).To(Equal([]string{
				"elasticsearch_exporter",
				"--es.uri=https://elastic.example.com:9200",
				"--es.ssl-skip-verify",
			}))
		})
	})

	Describe("with template vars in labels", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Labels = map[string]string{
				"prometheus":      "true",
				"prometheus_host": "{{ES_HOST}}",
			}
			env.Vars["ES_HOST"] = "elastic.example.com"
			client = newCreateMock("svc-tpl-labels")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should expand template vars in labels", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)
			labels := spec["Labels"].(map[string]string)
			Expect(labels["prometheus_host"]).To(Equal("elastic.example.com"))
			Expect(labels["prometheus"]).To(Equal("true"))
		})
	})

	Describe("with multiple template vars in one string", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Command = []string{"connect", "{{HOST}}:{{PORT}}"}
			env.Vars["HOST"] = "db.example.com"
			env.Vars["PORT"] = "5432"
			client = newCreateMock("svc-tpl-multi")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should expand multiple vars in one string", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			command := containerSpec["Command"].([]string)
			Expect(command[1]).To(Equal("db.example.com:5432"))
		})
	})

	Describe("with missing template var", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Command = []string{"run", "--uri={{MISSING_VAR}}"}
			client = newCreateMock("svc-tpl-missing")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("MISSING_VAR"))
			Expect(err.Error()).To(ContainSubstring("not found in env"))
		})
	})

	Describe("with missing template var in labels", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Labels = map[string]string{
				"host": "{{NOPE}}",
			}
			client = newCreateMock("svc-tpl-missing-label")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should return an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("NOPE"))
		})
	})

	Describe("with global template vars", func() {
		var globalVars map[string]string

		BeforeEach(func() {
			svc.Secrets = nil
			svc.Command = []string{"run", "--host={{VHOST}}"}
			svc.Labels = map[string]string{
				"external_host": "{{VHOST}}",
			}
			globalVars = map[string]string{
				"VHOST": "mon.example.com",
			}
			client = newCreateMock("svc-global")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, globalVars)
		})

		It("should expand global vars in command and labels", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)

			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			command := containerSpec["Command"].([]string)
			Expect(command[1]).To(Equal("--host=mon.example.com"))

			labels := spec["Labels"].(map[string]string)
			Expect(labels["external_host"]).To(Equal("mon.example.com"))
		})

		It("should not pass global vars to container env", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			envLines := containerSpec["Env"].([]string)
			for _, line := range envLines {
				Expect(line).NotTo(HavePrefix("VHOST="))
			}
		})
	})

	Describe("with global var overridden by service env", func() {
		var globalVars map[string]string

		BeforeEach(func() {
			svc.Secrets = nil
			svc.Command = []string{"run", "--host={{VHOST}}"}
			env.Vars["VHOST"] = "override.example.com"
			globalVars = map[string]string{
				"VHOST": "global.example.com",
			}
			client = newCreateMock("svc-override")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, globalVars)
		})

		It("should use service env value over global", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			command := containerSpec["Command"].([]string)
			Expect(command[1]).To(Equal("--host=override.example.com"))
		})
	})

	Describe("with env value containing template syntax", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Command = []string{"run", "{{TRICKY}}"}
			env.Vars["TRICKY"] = "has{{NESTED}}braces"
			client = newCreateMock("svc-tpl-tricky")
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
		})

		It("should not recursively expand values", func() {
			Expect(err).NotTo(HaveOccurred())

			calls := client.SendObjectCalls()
			createCall := findCall(calls, "POST", "/services/create")
			spec := createCall.Snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			command := containerSpec["Command"].([]string)
			Expect(command[1]).To(Equal("has{{NESTED}}braces"))
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
			deployer = swarm.New(client, nopLogger{})
		})

		JustBeforeEach(func() {
			id, created, err = deployRendered(ctx, deployer, svc, env, nil)
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

func deployRendered(ctx context.Context, deployer *swarm.Swarm, svc jed.Service, env jed.Env, vars map[string]string) (string, bool, error) {
	spec, err := jed.NewSpec(svc, env, vars)
	if err != nil {
		return "", false, err
	}
	return deployer.Deploy(ctx, spec)
}

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

var _ = Describe("GetService not found", func() {
	var (
		client   *ClientMock
		deployer *swarm.Swarm
		ctx      context.Context
		err      error
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &ClientMock{
			SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
				mockResponse([]map[string]any{}, rcv)
				return nil
			},
		}
		deployer = swarm.New(client, nopLogger{})
	})

	JustBeforeEach(func() {
		_, err = deployer.GetService(ctx, "missing")
	})

	It("should return ErrServiceNotFound", func() {
		Expect(err).To(HaveOccurred())
		Expect(errors.Is(err, swarm.ErrServiceNotFound)).To(BeTrue())
		Expect(err.Error()).To(ContainSubstring("missing"))
	})
})

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
		deployer = swarm.New(client, nopLogger{})
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
