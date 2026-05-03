package transship_test

import (
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/swarm"
	"github.com/clarktrimble/jed/transship"
	"github.com/pkg/errors"
)

func TestTransship(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Transship Suite")
}

var _ = Describe("Deployer", func() {
	var (
		ctx    context.Context
		store  *fakeStore
		client *fakeSwarmClient
		sw     *swarm.Swarm
		d      *transship.Deployer
		res    transship.DeployResult
		err    error
	)

	BeforeEach(func() {
		ctx = context.Background()
		store = newFakeStore()
		client = &fakeSwarmClient{}
		sw = swarm.New(client, nopLogger{})
		d = &transship.Deployer{Store: store, Swarm: sw}

		store.services["app"] = jed.Service{
			Name:    "app",
			Image:   "local/app:v1",
			Network: "svc-net",
			Command: []string{"serve", "--host={{VHOST}}", "--port={{PORT}}"},
		}
		store.envs["app"] = jed.Env{Name: "app", Vars: map[string]string{"PORT": "8080"}}
		store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com"}}
	})

	JustBeforeEach(func() {
		res, err = d.Deploy(ctx, "app")
	})

	When("the service does not exist in swarm", func() {
		BeforeEach(func() {
			client.serviceExists = false
			client.createID = "svc-new-1234567890"
		})

		It("creates the swarm service", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Created()).To(BeTrue())
			Expect(res.ID).To(Equal("svc-new-1234567890"))
		})

		It("returns the loaded service and envs", func() {
			Expect(res.Service.Name).To(Equal("app"))
			Expect(res.Env.Vars).To(HaveKeyWithValue("PORT", "8080"))
			Expect(res.Global.Vars).To(HaveKeyWithValue("VHOST", "app.example.com"))
		})

		It("passes global template vars to swarm deploy", func() {
			createCall := client.findCall("POST", "/services/create")
			Expect(createCall).NotTo(BeNil())

			spec := createCall.snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)

			Expect(containerSpec["Command"]).To(Equal([]string{"serve", "--host=app.example.com", "--port=8080"}))
			Expect(containerSpec["Env"]).To(ContainElement("PORT=8080"))
			Expect(containerSpec["Env"]).NotTo(ContainElement("VHOST=app.example.com"))
		})
	})

	When("the service already exists in swarm", func() {
		BeforeEach(func() {
			client.serviceExists = true
			client.version = 42
		})

		It("updates the swarm service", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(res.Created()).To(BeFalse())

			updateCall := client.findCall("POST", "/update")
			Expect(updateCall).NotTo(BeNil())
			Expect(updateCall.path).To(ContainSubstring("version=42"))
		})
	})

	When("a custom global env name is configured", func() {
		BeforeEach(func() {
			client.serviceExists = false
			client.createID = "svc-custom-global"
			store.envs["deploy"] = jed.Env{Name: "deploy", Vars: map[string]string{"VHOST": "deploy.example.com"}}
			d.GlobalEnvName = "deploy"
		})

		It("uses that env for template vars", func() {
			Expect(err).NotTo(HaveOccurred())

			createCall := client.findCall("POST", "/services/create")
			spec := createCall.snd.(map[string]any)
			taskTemplate := spec["TaskTemplate"].(map[string]any)
			containerSpec := taskTemplate["ContainerSpec"].(map[string]any)
			Expect(containerSpec["Command"]).To(ContainElement("--host=deploy.example.com"))
		})
	})

	When("the service is invalid", func() {
		BeforeEach(func() {
			store.services["app"] = jed.Service{Name: "app", Image: "local/app:v1"}
		})

		It("fails before calling swarm", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to validate service"))
			Expect(client.calls).To(BeEmpty())
		})
	})

	When("dependencies are nil", func() {
		It("rejects a nil store", func() {
			_, err := (&transship.Deployer{Swarm: sw}).Deploy(ctx, "app")
			Expect(err).To(MatchError("transship deployer has nil store"))
		})

		It("rejects a nil swarm", func() {
			_, err := (&transship.Deployer{Store: store}).Deploy(ctx, "app")
			Expect(err).To(MatchError("transship deployer has nil swarm"))
		})
	})
})

type fakeStore struct {
	services map[string]jed.Service
	envs     map[string]jed.Env
}

func newFakeStore() *fakeStore {
	return &fakeStore{
		services: map[string]jed.Service{},
		envs:     map[string]jed.Env{},
	}
}

func (s *fakeStore) GetService(ctx context.Context, name string) (jed.Service, error) {
	svc, ok := s.services[name]
	if !ok {
		return jed.Service{}, errors.Errorf("service not found: %s", name)
	}
	return svc, nil
}

func (s *fakeStore) SetService(ctx context.Context, svc jed.Service) error {
	s.services[svc.Name] = svc
	return nil
}

func (s *fakeStore) DelService(ctx context.Context, name string) error {
	delete(s.services, name)
	return nil
}

func (s *fakeStore) Services(ctx context.Context) ([]jed.Service, error) {
	services := make([]jed.Service, 0, len(s.services))
	for _, svc := range s.services {
		services = append(services, svc)
	}
	return services, nil
}

func (s *fakeStore) GetEnv(ctx context.Context, name string) (jed.Env, error) {
	env, ok := s.envs[name]
	if !ok {
		return jed.Env{Name: name, Vars: map[string]string{}}, nil
	}
	return env, nil
}

func (s *fakeStore) SetEnv(ctx context.Context, env jed.Env) error {
	s.envs[env.Name] = env
	return nil
}

func (s *fakeStore) DelEnv(ctx context.Context, name string) error {
	delete(s.envs, name)
	return nil
}

func (s *fakeStore) Envs(ctx context.Context) ([]jed.Env, error) {
	envs := make([]jed.Env, 0, len(s.envs))
	for _, env := range s.envs {
		envs = append(envs, env)
	}
	return envs, nil
}

type fakeSwarmClient struct {
	serviceExists bool
	version       int
	createID      string
	calls         []swarmCall
}

type swarmCall struct {
	method string
	path   string
	snd    any
}

func (c *fakeSwarmClient) SendObject(ctx context.Context, method, path string, snd, rcv any) error {
	c.calls = append(c.calls, swarmCall{method: method, path: path, snd: snd})

	switch {
	case method == "GET" && strings.Contains(path, "/services?"):
		if !c.serviceExists {
			return mockResponse([]map[string]any{}, rcv)
		}
		return mockResponse([]map[string]any{{"Version": map[string]any{"Index": c.version}}}, rcv)
	case method == "POST" && strings.Contains(path, "/services/create"):
		return mockResponse(map[string]string{"ID": c.createID}, rcv)
	case method == "POST" && strings.Contains(path, "/update"):
		return nil
	default:
		return nil
	}
}

func (c *fakeSwarmClient) SendJson(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	return nil, nil
}

func (c *fakeSwarmClient) StreamLines(ctx context.Context, path string) (<-chan []byte, error) {
	ch := make(chan []byte)
	close(ch)
	return ch, nil
}

func (c *fakeSwarmClient) findCall(method, pathSubstring string) *swarmCall {
	for i := range c.calls {
		if c.calls[i].method == method && strings.Contains(c.calls[i].path, pathSubstring) {
			return &c.calls[i]
		}
	}
	return nil
}

func mockResponse(obj, rcv any) error {
	if rcv == nil {
		return nil
	}
	data, err := json.Marshal(obj)
	if err != nil {
		return err
	}
	return json.Unmarshal(data, rcv)
}

type nopLogger struct{}

func (nopLogger) Info(ctx context.Context, msg string, kv ...any)             {}
func (nopLogger) Debug(ctx context.Context, msg string, kv ...any)            {}
func (nopLogger) Error(ctx context.Context, msg string, err error, kv ...any) {}
