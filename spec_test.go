package jed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/logger/loggertest"
)

var _ = Describe("Jed", func() {
	Describe("New", func() {
		It("loads render vars from the named env", func() {
			ctx := context.Background()
			store := newFakeStore()
			store.services["app"] = jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
				Command: []string{"--host={{VHOST}}"},
			}
			store.envs["app"] = jed.Env{Name: "app", Vars: map[string]string{}}
			store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com"}}

			j, err := jed.New(ctx, store, "_global", loggertest.NewLoggerMock())
			Expect(err).NotTo(HaveOccurred())

			spec, err := j.Spec(ctx, "app")
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"--host=app.example.com"}))
		})

		It("rejects a nil store", func() {
			_, err := jed.New(context.Background(), nil, "_global", loggertest.NewLoggerMock())
			Expect(err).To(MatchError("jed has nil store"))
		})

		It("rejects a nil logger", func() {
			_, err := jed.New(context.Background(), newFakeStore(), "_global", nil)
			Expect(err).To(MatchError("jed has nil logger"))
		})
	})

	Describe("Store", func() {
		It("returns the underlying store", func() {
			ctx := context.Background()
			store := newFakeStore()

			j, err := jed.New(ctx, store, "_global", loggertest.NewLoggerMock())
			Expect(err).NotTo(HaveOccurred())
			Expect(j.Store()).To(BeIdenticalTo(store))
		})

		It("returns nil for a nil Jed", func() {
			var j *jed.Jed
			Expect(j.Store()).To(BeNil())
		})
	})

	Describe("Scale", func() {
		var (
			ctx   context.Context
			store *fakeStore
			j     *jed.Jed
			err   error
		)

		BeforeEach(func() {
			ctx = context.Background()
			store = newFakeStore()
			store.services["app"] = jed.Service{
				Name:     "app",
				Image:    "local/app:v1",
				Network:  "svc-net",
				Replicas: 1,
			}
			j, err = jed.New(ctx, store, "_global", loggertest.NewLoggerMock())
			Expect(err).NotTo(HaveOccurred())
		})

		It("updates the stored replica count", func() {
			err := j.Scale(ctx, "app", 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.services["app"].Replicas).To(Equal(3))
		})

		It("allows scaling to zero", func() {
			err := j.Scale(ctx, "app", 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.services["app"].Replicas).To(Equal(0))
		})

		It("rejects negative replica counts", func() {
			err := j.Scale(ctx, "app", -1)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("replicas cannot be negative"))
			Expect(store.services["app"].Replicas).To(Equal(1))
		})

		It("returns store lookup errors", func() {
			err := j.Scale(ctx, "missing", 2)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("failed to get service"))
		})
	})

	Describe("Spec", func() {
		var (
			ctx   context.Context
			store *fakeStore
			j     *jed.Jed
			spec  jed.Spec
			err   error
		)

		BeforeEach(func() {
			ctx = context.Background()
			store = newFakeStore()
			store.services["app"] = jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
				Command: []string{"serve", "--host={{VHOST}}", "--port={{PORT}}"},
			}
			store.envs["app"] = jed.Env{Name: "app", Vars: map[string]string{"PORT": "8080"}}
			store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com"}}

			j, err = jed.New(ctx, store, "_global", loggertest.NewLoggerMock())
			Expect(err).NotTo(HaveOccurred())
		})

		JustBeforeEach(func() {
			spec, err = j.Spec(ctx, "app")
		})

		It("loads service and env and returns a rendered spec", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Name).To(Equal("app"))
			Expect(spec.Service.Command).To(Equal([]string{"serve", "--host=app.example.com", "--port=8080"}))
			Expect(spec.Env.Vars).To(HaveKeyWithValue("PORT", "8080"))
			Expect(spec.Env.Vars).NotTo(HaveKey("VHOST"))
		})

		It("reloads render vars from the store", func() {
			Expect(err).NotTo(HaveOccurred())

			store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "updated.example.com"}}
			spec, err = j.Spec(ctx, "app")
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"serve", "--host=updated.example.com", "--port=8080"}))
		})

		When("the service is invalid", func() {
			BeforeEach(func() {
				store.services["app"] = jed.Service{Name: "app", Image: "local/app:v1"}
			})

			It("fails validation", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to validate service"))
			})
		})
	})
})

var _ = Describe("Spec", func() {
	Describe("Render", func() {
		var (
			svc  jed.Service
			env  jed.Env
			vars map[string]string
		)

		BeforeEach(func() {
			svc = jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
				Command: []string{"serve", "--host={{VHOST}}", "--port={{PORT}}"},
				Labels: map[string]string{
					"host": "{{VHOST}}",
					"mode": "prod",
				},
			}
			env = jed.Env{Name: "app", Vars: map[string]string{"PORT": "8080"}}
			vars = map[string]string{"VHOST": "app.example.com"}
		})

		It("expands command vars", func() {
			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"serve", "--host=app.example.com", "--port=8080"}))
		})

		It("expands label vars", func() {
			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Labels).To(HaveKeyWithValue("host", "app.example.com"))
			Expect(spec.Service.Labels).To(HaveKeyWithValue("mode", "prod"))
		})

		It("expands link URL vars", func() {
			svc.About.Links = []jed.Link{
				{Text: "app", Url: "https://{{VHOST}}/dashboard?port={{PORT}}"},
			}

			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.About.Links).To(Equal([]jed.Link{
				{Text: "app", Url: "https://app.example.com/dashboard?port=8080"},
			}))
		})

		It("expands vars throughout service strings", func() {
			svc.Image = "postgres:{{PG_VERSION}}"
			svc.Network = "{{NETWORK}}"
			svc.User = "1000:{{DOCKER_GID}}"
			svc.Volumes = map[string]string{"{{CERTS_PATH}}": "/certs/{{CERT_NAME}}"}
			svc.Ports = map[string]string{"{{CONTAINER_PORT}}/tcp": "{{HOST_PORT}}"}
			svc.Secrets = []string{"{{SECRET_NAME}}"}
			svc.Configs = map[string]string{"{{CONFIG_NAME}}": "/etc/{{CONFIG_FILE}}"}
			svc.Hosts = []string{"{{HOST_IP}} {{HOST_NAME}}"}
			svc.Resources = jed.Resources{CPULimit: "{{CPU_LIMIT}}"}
			svc.Restart = "{{RESTART}}"
			svc.Traefik = &jed.Traefik{Port: "{{TRAEFIK_PORT}}"}
			svc.About = jed.About{
				Desc:  "{{DESC}}",
				Links: []jed.Link{{Text: "{{LINK_TEXT}}", Url: "https://{{VHOST}}"}},
				Notes: []jed.Note{{Author: "{{AUTHOR}}", Content: "{{NOTE}}"}},
			}
			vars = map[string]string{
				"AUTHOR":         "ops",
				"CERT_NAME":      "ca.pem",
				"CERTS_PATH":     "/srv/certs",
				"CONFIG_FILE":    "app.conf",
				"CONFIG_NAME":    "app_config",
				"CONTAINER_PORT": "5432",
				"CPU_LIMIT":      "1.0",
				"DESC":           "database",
				"DOCKER_GID":     "967",
				"HOST_IP":        "10.0.0.10",
				"HOST_NAME":      "db.local",
				"HOST_PORT":      "15432",
				"LINK_TEXT":      "dashboard",
				"NETWORK":        "prod-net",
				"NOTE":           "ready",
				"PG_VERSION":     "16",
				"RESTART":        "on-failure",
				"SECRET_NAME":    "db_password",
				"TRAEFIK_PORT":   "8080",
				"VHOST":          "app.example.com",
			}

			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Image).To(Equal("postgres:16"))
			Expect(spec.Service.Network).To(Equal("prod-net"))
			Expect(spec.Service.User).To(Equal("1000:967"))
			Expect(spec.Service.Volumes).To(HaveKeyWithValue("/srv/certs", "/certs/ca.pem"))
			Expect(spec.Service.Ports).To(HaveKeyWithValue("5432/tcp", "15432"))
			Expect(spec.Service.Secrets).To(Equal([]string{"db_password"}))
			Expect(spec.Service.Configs).To(HaveKeyWithValue("app_config", "/etc/app.conf"))
			Expect(spec.Service.Hosts).To(Equal([]string{"10.0.0.10 db.local"}))
			Expect(spec.Service.Resources.CPULimit).To(Equal("1.0"))
			Expect(spec.Service.Restart).To(Equal("on-failure"))
			Expect(spec.Service.Traefik.Port).To(Equal("8080"))
			Expect(spec.Service.About.Desc).To(Equal("database"))
			Expect(spec.Service.About.Links).To(Equal([]jed.Link{{Text: "dashboard", Url: "https://app.example.com"}}))
			Expect(spec.Service.About.Notes[0].Author).To(Equal("ops"))
			Expect(spec.Service.About.Notes[0].Content).To(Equal("ready"))
		})

		It("lets service env win over injected vars", func() {
			env.Vars["VHOST"] = "override.example.com"

			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(ContainElement("--host=override.example.com"))
			Expect(spec.Service.Labels).To(HaveKeyWithValue("host", "override.example.com"))
		})

		It("renders env values from injected vars without adding injected vars to Spec.Env", func() {
			env.Vars["FWD_URL"] = "https://{{VHOST}}/fwd"

			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Env.Vars).To(HaveKeyWithValue("PORT", "8080"))
			Expect(spec.Env.Vars).To(HaveKeyWithValue("FWD_URL", "https://app.example.com/fwd"))
			Expect(spec.Env.Vars).NotTo(HaveKey("VHOST"))
		})

		It("does not render env values from sibling env vars", func() {
			env.Vars["FWD_URL"] = "https://{{PORT}}/fwd"

			_, err := jed.Render(svc, env, vars)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("PORT"))
		})

		It("fails on missing vars", func() {
			delete(vars, "VHOST")

			_, err := jed.Render(svc, env, vars)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("VHOST"))
		})

		It("fails on missing link URL vars", func() {
			svc.Command = nil
			svc.Labels = nil
			svc.About.Links = []jed.Link{{Text: "app", Url: "https://{{VHOST}}"}}
			delete(vars, "VHOST")

			_, err := jed.Render(svc, env, vars)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("VHOST"))
		})

		It("expands in a single pass", func() {
			svc.Command = []string{"run", "{{TRICKY}}"}
			vars["TRICKY"] = "has{{NESTED}}braces"

			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"run", "has{{NESTED}}braces"}))
		})

		It("leaves unmatched opening braces unchanged", func() {
			svc.Command = []string{"run", "before {{VHOST"}

			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"run", "before {{VHOST"}))
		})
	})
})
