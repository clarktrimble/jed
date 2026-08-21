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
			err := store.SetService(ctx, jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
				Command: []string{"--host={{VHOST}}"},
				User:    "1000:{{DOCKER_GID}}",
				Groups:  []string{"{{DOCKER_GID}}"},
			})
			Expect(err).NotTo(HaveOccurred())
			store.intents["app"] = jed.Intent{Name: "app", Image: "local/app:v1"}
			store.envs["app"] = jed.Env{Name: "app", Vars: map[string]string{}}
			store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com", "DOCKER_GID": "967"}}

			j := (&jed.Config{VarsEnvName: "_global"}).New(store, loggertest.NewLoggerMock())

			spec, err := j.Spec(ctx, "app")
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"--host=app.example.com"}))
			Expect(spec.Service.User).To(Equal("1000:967"))
			Expect(spec.Service.Groups).To(Equal([]string{"967"}))
		})
	})

	Describe("Store", func() {
		It("returns the underlying store", func() {
			store := newFakeStore()

			j := (&jed.Config{VarsEnvName: "_global"}).New(store, loggertest.NewLoggerMock())
			Expect(j.Store()).To(BeIdenticalTo(store))
		})
	})

	Describe("Scale", func() {
		var (
			ctx   context.Context
			store *fakeStore
			j     *jed.Jed
		)

		BeforeEach(func() {
			ctx = context.Background()
			store = newFakeStore()
			store.intents["app"] = jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 1}
			j = (&jed.Config{VarsEnvName: "_global"}).New(store, loggertest.NewLoggerMock())
		})

		It("updates the stored intent replica count", func() {
			err := j.Scale(ctx, "app", 3)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.intents["app"].Replicas).To(Equal(3))
		})

		It("allows scaling to zero", func() {
			err := j.Scale(ctx, "app", 0)
			Expect(err).NotTo(HaveOccurred())
			Expect(store.intents["app"].Replicas).To(Equal(0))
		})

		It("rejects negative replica counts", func() {
			err := j.Scale(ctx, "app", -1)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("replicas cannot be negative"))
			Expect(store.intents["app"].Replicas).To(Equal(1))
		})

		It("returns missing intent errors", func() {
			err := j.Scale(ctx, "missing", 2)
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("intent not found"))
		})
	})

	Describe("Enable and Disable", func() {
		var (
			ctx   context.Context
			store *fakeStore
			j     *jed.Jed
		)

		BeforeEach(func() {
			ctx = context.Background()
			store = newFakeStore()
			store.services["app"] = jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
			}
			j = (&jed.Config{VarsEnvName: "_global"}).New(store, loggertest.NewLoggerMock())
		})

		It("sets the stored intent", func() {
			err := store.SetService(ctx, jed.Service{Name: "app", Image: "local/app:v1", Network: "svc-net"})
			Expect(err).NotTo(HaveOccurred())

			err = j.Enable(ctx, "app", "local/app:v1")
			Expect(err).NotTo(HaveOccurred())
			Expect(store.intents["app"]).To(Equal(jed.Intent{Name: "app", Image: "local/app:v1"}))
		})

		It("is idempotent when the existing intent has the same image", func() {
			err := store.SetService(ctx, jed.Service{Name: "app", Image: "local/app:v1", Network: "svc-net"})
			Expect(err).NotTo(HaveOccurred())
			store.intents["app"] = jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 2}

			err = j.Enable(ctx, "app", "local/app:v1")
			Expect(err).NotTo(HaveOccurred())
			Expect(store.intents["app"]).To(Equal(jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 2}))
		})

		It("switches image when the existing intent is stopped", func() {
			err := store.SetService(ctx, jed.Service{Name: "app", Image: "local/app:v2", Network: "svc-net"})
			Expect(err).NotTo(HaveOccurred())
			store.intents["app"] = jed.Intent{Name: "app", Image: "local/app:v1"}

			err = j.Enable(ctx, "app", "local/app:v2")
			Expect(err).NotTo(HaveOccurred())
			Expect(store.intents["app"]).To(Equal(jed.Intent{Name: "app", Image: "local/app:v2"}))
		})

		It("rejects switching image when the existing intent has replicas", func() {
			err := store.SetService(ctx, jed.Service{Name: "app", Image: "local/app:v2", Network: "svc-net"})
			Expect(err).NotTo(HaveOccurred())
			store.intents["app"] = jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 2}

			err = j.Enable(ctx, "app", "local/app:v2")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has 2 replicas"))
			Expect(store.intents["app"]).To(Equal(jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 2}))
		})

		It("returns service lookup errors", func() {
			err := j.Enable(ctx, "missing", "local/app:v1")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("service not found"))
		})

		It("removes a stopped stored intent", func() {
			store.intents["app"] = jed.Intent{Name: "app", Image: "local/app:v1"}

			err := j.Disable(ctx, "app")
			Expect(err).NotTo(HaveOccurred())
			Expect(store.intents).NotTo(HaveKey("app"))
		})

		It("is idempotent when the service has no intent", func() {
			err := j.Disable(ctx, "missing")
			Expect(err).NotTo(HaveOccurred())
		})

		It("rejects disabling an intent with replicas", func() {
			store.intents["app"] = jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 2}

			err := j.Disable(ctx, "app")
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("has 2 replicas"))
			Expect(store.intents["app"]).To(Equal(jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 2}))
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
			err = store.SetService(ctx, jed.Service{
				Name:    "app",
				Image:   "local/app:v1",
				Network: "svc-net",
				Command: []string{"serve", "--host={{VHOST}}", "--port={{PORT}}"},
			})
			Expect(err).NotTo(HaveOccurred())
			store.intents["app"] = jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 2}
			store.envs["app"] = jed.Env{Name: "app", Vars: map[string]string{"PORT": "8080"}}
			store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com"}}

			j = (&jed.Config{VarsEnvName: "_global"}).New(store, loggertest.NewLoggerMock())
		})

		JustBeforeEach(func() {
			spec, err = j.Spec(ctx, "app")
		})

		It("loads the intent, service, and env and returns a rendered spec", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Intent).To(Equal(jed.Intent{Name: "app", Image: "local/app:v1", Replicas: 2}))
			Expect(spec.Service.Name).To(Equal("app"))
			Expect(spec.Service.Image).To(Equal("local/app:v1"))
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

		It("applies service defaults", func() {
			svc, err := store.GetService(ctx, "app", "local/app:v1")
			Expect(err).NotTo(HaveOccurred())
			svc.Network = ""
			svc.User = ""
			err = store.SetService(ctx, svc)
			Expect(err).NotTo(HaveOccurred())

			j = (&jed.Config{VarsEnvName: "_global", DefaultUid: "1000", DefaultNetwork: "svc-net"}).New(store, loggertest.NewLoggerMock())
			spec, err = j.Spec(ctx, "app")
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.User).To(Equal("1000"))
			Expect(spec.Service.Network).To(Equal("svc-net"))
		})

		When("the service is invalid", func() {
			BeforeEach(func() {
				err = store.SetService(ctx, jed.Service{Name: "app", Image: "local/app:v1", Restart: "sometimes"})
				Expect(err).NotTo(HaveOccurred())
			})

			It("fails validation", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("failed to validate service"))
				Expect(err.Error()).To(ContainSubstring("restart"))
			})
		})

		When("the service has no intent", func() {
			BeforeEach(func() {
				delete(store.intents, "app")
			})

			It("returns the missing intent error", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("intent not found"))
			})
		})

		When("the service has local volumes", func() {
			BeforeEach(func() {
				svc, err := store.GetService(ctx, "app", "local/app:v1")
				Expect(err).NotTo(HaveOccurred())
				svc.LocalVolumes = []string{"/qkview-history", "/var/lib/app"}
				err = store.SetService(ctx, svc)
				Expect(err).NotTo(HaveOccurred())
				store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com", "LOCAL_ROOT": "/opt/bastille"}}
			})

			It("adds convention-derived bind mounts to volumes and clears local volumes", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Service.Volumes).To(HaveKeyWithValue("/opt/bastille/app/_qkview-history", "/qkview-history"))
				Expect(spec.Service.Volumes).To(HaveKeyWithValue("/opt/bastille/app/_var_lib_app", "/var/lib/app"))
				Expect(spec.Service.LocalVolumes).To(BeNil())
			})

			When("a local volume has a trailing slash", func() {
				BeforeEach(func() {
					svc, err := store.GetService(ctx, "app", "local/app:v1")
					Expect(err).NotTo(HaveOccurred())
					svc.LocalVolumes = []string{"/data/"}
					err = store.SetService(ctx, svc)
					Expect(err).NotTo(HaveOccurred())
				})

				It("uses the cleaned target", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.Service.Volumes).To(HaveKeyWithValue("/opt/bastille/app/_data", "/data"))
				})
			})

			When("the whole local volume path is templated", func() {
				BeforeEach(func() {
					svc, err := store.GetService(ctx, "app", "local/app:v1")
					Expect(err).NotTo(HaveOccurred())
					svc.LocalVolumes = []string{"{{VOL_PATH}}"}
					err = store.SetService(ctx, svc)
					Expect(err).NotTo(HaveOccurred())
					store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com", "LOCAL_ROOT": "/opt/bastille", "VOL_PATH": "/qkview-history"}}
				})

				It("validates and resolves after render", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.Service.Volumes).To(HaveKeyWithValue("/opt/bastille/app/_qkview-history", "/qkview-history"))
				})
			})

			When("LOCAL_ROOT is not safe", func() {
				BeforeEach(func() {
					store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com", "LOCAL_ROOT": "/opt/../bastille"}}
				})

				It("returns an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("LOCAL_ROOT"))
					Expect(err.Error()).To(ContainSubstring("cannot contain . or .."))
				})
			})

			When("LOCAL_ROOT is missing", func() {
				BeforeEach(func() {
					store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com"}}
				})

				It("returns an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("LOCAL_ROOT is required"))
				})
			})

			When("a rendered local volume contains a dot segment", func() {
				BeforeEach(func() {
					svc, err := store.GetService(ctx, "app", "local/app:v1")
					Expect(err).NotTo(HaveOccurred())
					svc.LocalVolumes = []string{"/{{VOL_PATH}}"}
					err = store.SetService(ctx, svc)
					Expect(err).NotTo(HaveOccurred())
					store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com", "LOCAL_ROOT": "/opt/bastille", "VOL_PATH": "foo/../data"}}
				})

				It("returns an error", func() {
					Expect(err).To(HaveOccurred())
					Expect(err.Error()).To(ContainSubstring("cannot contain . or .."))
				})
			})
		})

		When("a local volume conflicts with an explicit volume", func() {
			var lgr *loggertest.LoggerMock

			BeforeEach(func() {
				svc, err := store.GetService(ctx, "app", "local/app:v1")
				Expect(err).NotTo(HaveOccurred())
				svc.Volumes = map[string]string{"/explicit/path": "/qkview-history"}
				svc.LocalVolumes = []string{"/qkview-history"}
				err = store.SetService(ctx, svc)
				Expect(err).NotTo(HaveOccurred())
				store.envs["_global"] = jed.Env{Name: "_global", Vars: map[string]string{"VHOST": "app.example.com", "LOCAL_ROOT": "/opt/bastille"}}

				lgr = loggertest.NewLoggerMock()
				j = (&jed.Config{VarsEnvName: "_global"}).New(store, lgr)
			})

			It("keeps the explicit volume, clears local volumes, and logs an error", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Service.Volumes).To(HaveKeyWithValue("/explicit/path", "/qkview-history"))
				Expect(spec.Service.Volumes).NotTo(HaveKey("/opt/bastille/app/_qkview-history"))
				Expect(spec.Service.LocalVolumes).To(BeNil())
				Expect(lgr.ErrorCalls()).To(HaveLen(1))
			})
		})

		When("a default uid is configured", func() {
			BeforeEach(func() {
				j = (&jed.Config{VarsEnvName: "_global", DefaultUid: "1000"}).New(store, loggertest.NewLoggerMock())
			})

			It("applies the default uid when the service omits user", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Service.User).To(Equal("1000"))
			})

			When("the service sets its own user", func() {
				BeforeEach(func() {
					svc, err := store.GetService(ctx, "app", "local/app:v1")
					Expect(err).NotTo(HaveOccurred())
					svc.User = "1500:1600"
					err = store.SetService(ctx, svc)
					Expect(err).NotTo(HaveOccurred())
				})

				It("keeps the service user", func() {
					Expect(err).NotTo(HaveOccurred())
					Expect(spec.Service.User).To(Equal("1500:1600"))
				})
			})
		})
	})

	Describe("SpecWithEnv", func() {
		var (
			ctx   context.Context
			store *fakeStore
			j     *jed.Jed
			svc   jed.Service
			env   jed.Env
			vars  map[string]string
			spec  jed.Spec
			err   error
		)

		BeforeEach(func() {
			ctx = context.Background()
			store = newFakeStore()
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

			j = (&jed.Config{VarsEnvName: "_global"}).New(store, loggertest.NewLoggerMock())
		})

		JustBeforeEach(func() {
			err = store.SetService(ctx, svc)
			Expect(err).NotTo(HaveOccurred())
			store.intents["app"] = jed.Intent{Name: "app", Image: svc.Image, Replicas: 2}
			store.envs["_global"] = jed.Env{Name: "_global", Vars: vars}
			spec, err = j.SpecWithEnv(ctx, "app", env)
		})

		It("renders using the provided env and stored render vars", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"serve", "--host=app.example.com", "--port=8080"}))
			Expect(spec.Env.Vars).To(HaveKeyWithValue("PORT", "8080"))
		})

		When("a different env is stored for the service", func() {
			BeforeEach(func() {
				store.envs["app"] = jed.Env{Name: "app", Vars: map[string]string{"PORT": "1111"}}
			})

			It("uses the provided env and does not save it", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Service.Command).To(ContainElement("--port=8080"))
				Expect(store.envs["app"].Vars).To(HaveKeyWithValue("PORT", "1111"))
			})
		})

		It("expands label vars", func() {
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Labels).To(HaveKeyWithValue("host", "app.example.com"))
			Expect(spec.Service.Labels).To(HaveKeyWithValue("mode", "prod"))
		})

		When("the service has link URLs with vars", func() {
			BeforeEach(func() {
				svc.About.Links = []jed.Link{
					{Text: "app", Url: "https://{{VHOST}}/dashboard?port={{PORT}}"},
				}
			})

			It("expands link URL vars", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Service.About.Links).To(Equal([]jed.Link{
					{Text: "app", Url: "https://app.example.com/dashboard?port=8080"},
				}))
			})
		})

		When("service strings throughout reference vars", func() {
			BeforeEach(func() {
				svc.Image = "postgres:{{PG_VERSION}}"
				svc.Network = "{{NETWORK}}"
				svc.User = "1000:{{DOCKER_GID}}"
				svc.Volumes = map[string]string{"{{CERTS_PATH}}": "/certs/{{CERT_NAME}}"}
				svc.Ports = map[string]string{"{{CONTAINER_PORT}}/tcp": "{{HOST_PORT}}"}
				svc.Secrets = []string{"{{SECRET_NAME}}"}
				svc.Configs = map[string]string{"{{CONFIG_NAME}}": "/etc/{{CONFIG_FILE}}"}
				svc.Hosts = []string{"{{HOST_IP}} {{HOST_NAME}}"}
				svc.Resources = jed.Resources{CPULimit: "{{CPU_LIMIT}}"}
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
					"SECRET_NAME":    "db_password",
					"TRAEFIK_PORT":   "8080",
					"VHOST":          "app.example.com",
				}
			})

			It("expands vars throughout service strings", func() {
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
				Expect(spec.Service.Traefik.Port).To(Equal("8080"))
				Expect(spec.Service.About.Desc).To(Equal("database"))
				Expect(spec.Service.About.Links).To(Equal([]jed.Link{{Text: "dashboard", Url: "https://app.example.com"}}))
				Expect(spec.Service.About.Notes[0].Author).To(Equal("ops"))
				Expect(spec.Service.About.Notes[0].Content).To(Equal("ready"))
			})
		})

		When("the provided env overrides an injected var", func() {
			BeforeEach(func() {
				env.Vars["VHOST"] = "override.example.com"
			})

			It("lets the provided env win over injected vars", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Service.Command).To(ContainElement("--host=override.example.com"))
				Expect(spec.Service.Labels).To(HaveKeyWithValue("host", "override.example.com"))
			})
		})

		When("a provided env value references an injected var", func() {
			BeforeEach(func() {
				env.Vars["FWD_URL"] = "https://{{VHOST}}/fwd"
			})

			It("renders env values from injected vars without adding them to Spec.Env", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Env.Vars).To(HaveKeyWithValue("PORT", "8080"))
				Expect(spec.Env.Vars).To(HaveKeyWithValue("FWD_URL", "https://app.example.com/fwd"))
				Expect(spec.Env.Vars).NotTo(HaveKey("VHOST"))
			})
		})

		When("a provided env value references a sibling env var", func() {
			BeforeEach(func() {
				env.Vars["FWD_URL"] = "https://{{PORT}}/fwd"
			})

			It("does not render env values from sibling env vars", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("PORT"))
			})
		})

		When("a template var is missing", func() {
			BeforeEach(func() {
				delete(vars, "VHOST")
			})

			It("fails to render", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("VHOST"))
			})
		})

		When("a link URL var is missing", func() {
			BeforeEach(func() {
				svc.Command = nil
				svc.Labels = nil
				svc.About.Links = []jed.Link{{Text: "app", Url: "https://{{VHOST}}"}}
				delete(vars, "VHOST")
			})

			It("fails to render", func() {
				Expect(err).To(HaveOccurred())
				Expect(err.Error()).To(ContainSubstring("VHOST"))
			})
		})

		When("a var expands to text containing braces", func() {
			BeforeEach(func() {
				svc.Command = []string{"run", "{{TRICKY}}"}
				vars["TRICKY"] = "has{{NESTED}}braces"
			})

			It("expands in a single pass", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Service.Command).To(Equal([]string{"run", "has{{NESTED}}braces"}))
			})
		})

		When("a string has an unmatched opening brace", func() {
			BeforeEach(func() {
				svc.Command = []string{"run", "before {{VHOST"}
			})

			It("leaves unmatched opening braces unchanged", func() {
				Expect(err).NotTo(HaveOccurred())
				Expect(spec.Service.Command).To(Equal([]string{"run", "before {{VHOST"}))
			})
		})
	})
})
