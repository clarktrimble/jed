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

var _ = Describe("Spec", func() {
	var (
		client   *ClientMock
		deployer *swarm.Swarm
		ctx      context.Context
		svc      jed.Service
		env      jed.Env
		intent   jed.Intent
		body     swarm.Spec
		err      error
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
		intent = jed.Intent{Name: "reauth-acp", Image: "local/reauth-acp:a1b2c3d"}
		client = newSpecMock()
		deployer = swarm.New(client, nopLogger{})
	})

	JustBeforeEach(func() {
		body, err = deployer.Spec(ctx, jed.Spec{Service: svc, Env: env, Intent: intent})
	})

	It("builds the swarm service spec without creating or updating", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(body.Name).To(Equal("reauth-acp"))

		calls := client.SendObjectCalls()
		Expect(calls).To(HaveLen(1))
		Expect(calls[0].Method).To(Equal("GET"))
		Expect(calls[0].Path).To(ContainSubstring("/secrets"))
	})

	It("includes service configuration", func() {
		Expect(err).NotTo(HaveOccurred())

		taskTemplate := body.TaskTemplate
		containerSpec := taskTemplate.ContainerSpec
		Expect(containerSpec.Image).To(Equal("local/reauth-acp:a1b2c3d"))

		Expect(containerSpec.Env).To(ContainElement("RTH_CLIENT_BASEURI=https://10.35.44.62"))
		Expect(containerSpec.Env).To(ContainElement("RTH_SERVER_PORT=3031"))

		mounts := containerSpec.Mounts
		Expect(mounts).To(HaveLen(1))
		Expect(mounts[0].Source).To(Equal("svc-data"))
		Expect(mounts[0].Target).To(Equal("/data"))

		ports := body.EndpointSpec.Ports
		Expect(ports).To(HaveLen(1))
		Expect(ports[0].TargetPort).To(Equal(3031))
		Expect(ports[0].PublishedPort).To(Equal(8012))
		Expect(ports[0].Protocol).To(Equal("tcp"))

		networks := taskTemplate.Networks
		Expect(networks).To(HaveLen(1))
		Expect(networks[0].Target).To(Equal("svc-net"))
	})

	It("uses intent replicas", func() {
		intent.Replicas = 2

		body, err = deployer.Spec(ctx, jed.Spec{Service: svc, Env: env, Intent: intent})
		Expect(err).NotTo(HaveOccurred())
		Expect(body.Mode.Replicated.Replicas).To(Equal(2))
	})

	It("resolves latest swarm secrets", func() {
		Expect(err).NotTo(HaveOccurred())

		containerSpec := body.TaskTemplate.ContainerSpec
		secrets := containerSpec.Secrets
		Expect(secrets).To(HaveLen(1))
		Expect(secrets[0].SecretID).To(Equal("sec-abc-123"))
		Expect(secrets[0].SecretName).To(Equal("aruba_client_secret_v2"))
	})

	Describe("with invalid service", func() {
		BeforeEach(func() {
			svc.Restart = "sometimes"
			client = &ClientMock{}
			deployer = swarm.New(client, nopLogger{})
		})

		It("fails before calling Docker", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to validate service"))
			Expect(err.Error()).To(ContainSubstring("restart"))
			Expect(client.SendObjectCalls()).To(BeEmpty())
		})
	})

	Describe("with no secrets", func() {
		BeforeEach(func() {
			svc.Secrets = nil
		})

		It("does not include Secrets", func() {
			Expect(err).NotTo(HaveOccurred())

			containerSpec := body.TaskTemplate.ContainerSpec
			Expect(containerSpec.Secrets).To(BeEmpty())
		})
	})

	Describe("with default restart policy", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Restart = ""
		})

		It("disables restarts", func() {
			Expect(err).NotTo(HaveOccurred())

			restart := body.TaskTemplate.RestartPolicy
			Expect(restart).To(Equal(swarm.RestartPolicy{Condition: "none"}))
		})
	})

	Describe("with on-failure restart policy", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Restart = jed.RestartOnFailure
		})

		It("includes restart details", func() {
			Expect(err).NotTo(HaveOccurred())

			restart := body.TaskTemplate.RestartPolicy
			Expect(restart.Condition).To(Equal("on-failure"))
			Expect(restart.Delay).To(Equal(int64(5000000000)))
			Expect(restart.MaxAttempts).To(Equal(1))
		})
	})

	Describe("with configs", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Configs = map[string]string{
				"reauth_config": "/etc/reauth/config.yaml",
			}
		})

		It("includes resolved configs", func() {
			Expect(err).NotTo(HaveOccurred())

			containerSpec := body.TaskTemplate.ContainerSpec
			configs := containerSpec.Configs
			Expect(configs).To(HaveLen(1))
			Expect(configs[0].ConfigID).To(Equal("cfg-abc-123"))
			Expect(configs[0].ConfigName).To(Equal("reauth_config_v3"))

			Expect(configs[0].File.Name).To(Equal("/etc/reauth/config.yaml"))
		})
	})

	Describe("with hosts", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Hosts = []string{"192.168.88.75 vilnius", "10.0.0.1 gateway"}
		})

		It("includes Hosts in container spec", func() {
			Expect(err).NotTo(HaveOccurred())

			containerSpec := body.TaskTemplate.ContainerSpec
			hosts := containerSpec.Hosts
			Expect(hosts).To(HaveLen(2))
			Expect(hosts).To(ContainElement("192.168.88.75 vilnius"))
			Expect(hosts).To(ContainElement("10.0.0.1 gateway"))
		})
	})

	Describe("with groups", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Groups = []string{"967", "968"}
		})

		It("includes Groups in container spec", func() {
			Expect(err).NotTo(HaveOccurred())

			containerSpec := body.TaskTemplate.ContainerSpec
			groups := containerSpec.Groups
			Expect(groups).To(Equal([]string{"967", "968"}))
		})
	})

	Describe("with no groups", func() {
		BeforeEach(func() {
			svc.Secrets = nil
		})

		It("does not include Groups", func() {
			Expect(err).NotTo(HaveOccurred())

			containerSpec := body.TaskTemplate.ContainerSpec
			Expect(containerSpec.Groups).To(BeEmpty())
		})
	})

	Describe("marshaled JSON wire format", func() {
		BeforeEach(func() {
			svc.Secrets = nil
		})

		It("omits empty optional fields but keeps a zero replica count", func() {
			Expect(err).NotTo(HaveOccurred())

			raw, merr := json.Marshal(body)
			Expect(merr).NotTo(HaveOccurred())

			var decoded map[string]any
			Expect(json.Unmarshal(raw, &decoded)).To(Succeed())

			containerSpec := decoded["TaskTemplate"].(map[string]any)["ContainerSpec"].(map[string]any)
			Expect(containerSpec).NotTo(HaveKey("Groups"))
			Expect(containerSpec).NotTo(HaveKey("Secrets"))

			// Replicas has no omitempty: a legitimate scaled-to-zero must survive
			// the wire, so the key is present even when the value is 0.
			replicated := decoded["Mode"].(map[string]any)["Replicated"].(map[string]any)
			Expect(replicated).To(HaveKey("Replicas"))
			Expect(replicated["Replicas"]).To(BeEquivalentTo(0))
		})
	})

	Describe("with publish_mode host", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.PublishMode = "host"
		})

		It("includes PublishMode in port spec", func() {
			Expect(err).NotTo(HaveOccurred())

			ports := body.EndpointSpec.Ports
			Expect(ports).To(HaveLen(1))
			Expect(ports[0].PublishMode).To(Equal("host"))
		})
	})

	Describe("with custom user", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.User = "1000:967"
		})

		It("uses custom user in container spec", func() {
			Expect(err).NotTo(HaveOccurred())

			containerSpec := body.TaskTemplate.ContainerSpec
			Expect(containerSpec.User).To(Equal("1000:967"))
		})
	})

	Describe("with traefik config", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080"}
		})

		It("generates traefik labels", func() {
			Expect(err).NotTo(HaveOccurred())

			labels := body.Labels
			Expect(labels["traefik.enable"]).To(Equal("true"))
			Expect(labels["traefik.http.routers.reauth-acp.rule"]).To(Equal("PathPrefix(`/reauth-acp`)"))
			Expect(labels["traefik.http.routers.reauth-acp.entrypoints"]).To(Equal("websecure"))
			Expect(labels["traefik.http.routers.reauth-acp.tls"]).To(Equal("true"))
			Expect(labels["traefik.http.routers.reauth-acp.middlewares"]).To(Equal("ingress-auth,reauth-acp-strip"))
			Expect(labels["traefik.http.middlewares.reauth-acp-strip.stripprefix.prefixes"]).To(Equal("/reauth-acp"))
			Expect(labels["traefik.http.services.reauth-acp.loadbalancer.server.port"]).To(Equal("8080"))
		})
	})

	Describe("with traefik config preserving the path prefix", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080", PreservePathPrefix: true}
		})

		It("includes auth but not strip middleware", func() {
			Expect(err).NotTo(HaveOccurred())

			labels := body.Labels
			Expect(labels["traefik.enable"]).To(Equal("true"))
			Expect(labels["traefik.http.routers.reauth-acp.rule"]).To(Equal("PathPrefix(`/reauth-acp`)"))
			Expect(labels["traefik.http.routers.reauth-acp.middlewares"]).To(Equal("ingress-auth"))
			Expect(labels).NotTo(HaveKey("traefik.http.middlewares.reauth-acp-strip.stripprefix.prefixes"))
		})
	})

	Describe("with traefik config bypassing auth", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080", BypassAuth: true}
		})

		It("includes strip but not auth middleware", func() {
			Expect(err).NotTo(HaveOccurred())

			labels := body.Labels
			Expect(labels["traefik.http.routers.reauth-acp.middlewares"]).To(Equal("reauth-acp-strip"))
			Expect(labels["traefik.http.middlewares.reauth-acp-strip.stripprefix.prefixes"]).To(Equal("/reauth-acp"))
		})
	})

	Describe("with traefik config bypassing auth and preserving the path prefix", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080", BypassAuth: true, PreservePathPrefix: true}
		})

		It("does not include middleware", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(body.Labels).NotTo(HaveKey("traefik.http.routers.reauth-acp.middlewares"))
		})
	})

	Describe("with traefik config and explicit labels", func() {
		BeforeEach(func() {
			svc.Secrets = nil
			svc.Traefik = &jed.Traefik{Port: "8080"}
			svc.Labels = map[string]string{
				"custom.label":   "custom-value",
				"traefik.enable": "false",
				"traefik.http.routers.reauth-acp.middlewares": "",
			}
		})

		It("merges labels with explicit winning", func() {
			Expect(err).NotTo(HaveOccurred())

			labels := body.Labels
			Expect(labels["custom.label"]).To(Equal("custom-value"))
			Expect(labels["traefik.enable"]).To(Equal("false"))
			Expect(labels["traefik.http.routers.reauth-acp.middlewares"]).To(Equal(""))
			Expect(labels["traefik.http.routers.reauth-acp.rule"]).To(Equal("PathPrefix(`/reauth-acp`)"))
		})
	})
})

var _ = Describe("network volumes", func() {
	var (
		client *ClientMock
		input  jed.Spec
		body   swarm.Spec
		err    error
	)
	wantNetworkMount := map[string]any{
		"Type": "volume", "Source": "postgres_data_d2546c", "Target": "/var/lib/postgresql/data",
		"VolumeOptions": map[string]any{"DriverConfig": map[string]any{
			"Name":    "local",
			"Options": map[string]any{"type": "nfs", "o": "addr=nas,nfsvers=3,rw", "device": ":/exports/jed/postgres_data_d2546c"},
		}},
	}
	wireMounts := func() []map[string]any {
		data, marshalErr := json.Marshal(body)
		Expect(marshalErr).NotTo(HaveOccurred())
		var wire struct {
			TaskTemplate struct {
				ContainerSpec struct{ Mounts []map[string]any }
			}
		}
		Expect(json.Unmarshal(data, &wire)).To(Succeed())
		return wire.TaskTemplate.ContainerSpec.Mounts
	}
	BeforeEach(func() {
		client = newSpecMock()
		input = jed.Spec{
			Service:       jed.Service{Name: "postgres", Image: "postgres:16", Network: "svc-net", User: "1000"},
			NetworkMounts: []jed.NetworkMount{{Name: "postgres_data_d2546c", Target: "/var/lib/postgresql/data", Server: "nas", Export: "/exports/jed/postgres_data_d2546c"}},
		}
	})
	JustBeforeEach(func() { body, err = swarm.New(client, nopLogger{}).Spec(context.Background(), input) })

	It("emits the complete Docker NFSv3 mount", func() {
		Expect(err).NotTo(HaveOccurred())
		Expect(wireMounts()).To(ConsistOf(wantNetworkMount))
	})

	When("there are existing mounts", func() {
		BeforeEach(func() {
			input.Service.Volumes = map[string]string{"/srv/certs": "/certs", "ordinary-data": "/ordinary"}
		})
		It("keeps existing mounts without adding driver options to them", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(wireMounts()).To(ConsistOf(
				wantNetworkMount,
				map[string]any{"Type": "bind", "Source": "/srv/certs", "Target": "/certs"},
				map[string]any{"Type": "volume", "Source": "ordinary-data", "Target": "/ordinary"},
			))
		})
	})

	When("there are no network declarations", func() {
		BeforeEach(func() {
			input.NetworkMounts = nil
			input.Service.Volumes = map[string]string{"ordinary-data": "/ordinary"}
		})
		It("preserves the ordinary mount wire format", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(wireMounts()).To(ConsistOf(map[string]any{"Type": "volume", "Source": "ordinary-data", "Target": "/ordinary"}))
		})
	})

	When("network declarations have not been resolved", func() {
		BeforeEach(func() {
			input.NetworkMounts = nil
			input.Service.NetworkVolumes = []string{"/var/lib/postgresql/data"}
			input.Service.Secrets = []string{"db-password"}
			input.Service.Configs = map[string]string{"app-config": "/etc/app.conf"}
		})
		It("rejects unresolved network declarations before Docker lookups", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("unresolved network volumes"))
			Expect(client.SendObjectCalls()).To(BeEmpty())
			Expect(client.SendJsonCalls()).To(BeEmpty())
		})
	})
})

func newSpecMock() *ClientMock {
	return &ClientMock{
		SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
			switch {
			case method == "GET" && strings.Contains(path, "/secrets"):
				mockResponse([]secretItem{
					{ID: "sec-abc-123", Spec: specName{Name: "aruba_client_secret_v2"}},
					{ID: "sec-abc-100", Spec: specName{Name: "aruba_client_secret_v1"}},
				}, rcv)
				return nil
			case method == "GET" && strings.Contains(path, "/configs"):
				mockResponse([]secretItem{
					{ID: "cfg-abc-123", Spec: specName{Name: "reauth_config_v3"}},
					{ID: "cfg-abc-100", Spec: specName{Name: "reauth_config_v1"}},
				}, rcv)
				return nil
			default:
				return errors.New("unexpected Docker call")
			}
		},
	}
}
