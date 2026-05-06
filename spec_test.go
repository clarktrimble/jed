package jed_test

import (
	"context"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
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

			j, err := jed.New(ctx, store, "_global")
			Expect(err).NotTo(HaveOccurred())

			spec, err := j.Spec(ctx, "app")
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(Equal([]string{"--host=app.example.com"}))
		})

		It("rejects a nil store", func() {
			_, err := jed.New(context.Background(), nil, "_global")
			Expect(err).To(MatchError("jed has nil store"))
		})
	})

	Describe("Store", func() {
		It("returns the underlying store", func() {
			ctx := context.Background()
			store := newFakeStore()

			j, err := jed.New(ctx, store, "_global")
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
			j, err = jed.New(ctx, store, "_global")
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

			j, err = jed.New(ctx, store, "_global")
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

		It("lets service env win over injected vars", func() {
			env.Vars["VHOST"] = "override.example.com"

			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Service.Command).To(ContainElement("--host=override.example.com"))
			Expect(spec.Service.Labels).To(HaveKeyWithValue("host", "override.example.com"))
		})

		It("does not add injected vars to Spec.Env", func() {
			spec, err := jed.Render(svc, env, vars)
			Expect(err).NotTo(HaveOccurred())
			Expect(spec.Env.Vars).To(HaveKeyWithValue("PORT", "8080"))
			Expect(spec.Env.Vars).NotTo(HaveKey("VHOST"))
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
			env.Vars["TRICKY"] = "has{{NESTED}}braces"

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
