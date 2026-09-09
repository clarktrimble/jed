package jed_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/logger/loggertest"
	"github.com/clarktrimble/jed/store/memo"
)

var _ = Describe("API", func() {
	var (
		j   *jed.Jed
		rtr *http.ServeMux
		res *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		j = (&jed.Config{VarsEnvName: "_vars"}).New(memo.New(), loggertest.NewLoggerMock())

		rtr = http.NewServeMux()
		j.Register(rtr)
	})

	Describe("export route", func() {
		It("exports all stored data in deterministic order", func() {
			ctx := context.Background()
			store := j.Store()
			Expect(store.SetService(ctx, jed.Service{Name: "web", Image: "nginx:latest"})).To(Succeed())
			Expect(store.SetService(ctx, jed.Service{Name: "db", Image: "postgres:16"})).To(Succeed())
			Expect(store.SetService(ctx, jed.Service{Name: "web", Image: "alpine:latest"})).To(Succeed())
			Expect(store.SetEnv(ctx, jed.Env{Name: "web", Vars: map[string]string{"PORT": "8080"}})).To(Succeed())
			Expect(store.SetEnv(ctx, jed.Env{Name: "_global", Vars: map[string]string{"DOMAIN": "example.com"}})).To(Succeed())
			Expect(store.SetIntent(ctx, jed.Intent{Name: "web", Image: "nginx:latest", Replicas: 2})).To(Succeed())
			Expect(store.SetIntent(ctx, jed.Intent{Name: "db", Image: "postgres:16"})).To(Succeed())

			res = doRequest(rtr, http.MethodGet, "/store/export", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			var got struct {
				Schema   string        `json:"schema"`
				Services []jed.Service `json:"services"`
				Envs     []jed.Env     `json:"envs"`
				Intents  []jed.Intent  `json:"intents"`
			}
			decodeJSON(res, &got)
			Expect(got.Schema).To(Equal(jed.DBSchemaVersion))
			Expect(got.Services).To(Equal([]jed.Service{
				{Name: "db", Image: "postgres:16"},
				{Name: "web", Image: "alpine:latest"},
				{Name: "web", Image: "nginx:latest"},
			}))
			Expect(got.Envs).To(Equal([]jed.Env{
				{Name: "_global", Vars: map[string]string{"DOMAIN": "example.com"}},
				{Name: "web", Vars: map[string]string{"PORT": "8080"}},
			}))
			Expect(got.Intents).To(Equal([]jed.Intent{
				{Name: "db", Image: "postgres:16"},
				{Name: "web", Image: "nginx:latest", Replicas: 2},
			}))
		})

		It("exports empty arrays for an empty store", func() {
			res = doRequest(rtr, http.MethodGet, "/store/export", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			var got map[string]json.RawMessage
			decodeJSON(res, &got)
			Expect(string(got["services"])).To(Equal("[]"))
			Expect(string(got["envs"])).To(Equal("[]"))
			Expect(string(got["intents"])).To(Equal("[]"))
		})
	})

	Describe("import route", func() {
		It("imports into an empty store and verifies intent service references", func() {
			payload := map[string]any{
				"schema": jed.DBSchemaVersion,
				"services": []jed.Service{
					{Name: "web", Integration: &jed.Integration{Name: "frontend", Version: "v1.2.3"}, Image: "nginx:latest", Network: "backend", Ports: map[string]string{}, Labels: map[string]string{}, Volumes: map[string]string{}},
				},
				"envs": []jed.Env{
					{Name: "web"},
				},
				"intents": []jed.Intent{
					{Name: "web", Image: "nginx:latest", Replicas: 2},
				},
			}

			res = doJSON(rtr, http.MethodPut, "/store/import", payload)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			var summary map[string]int
			decodeJSON(res, &summary)
			Expect(summary).To(Equal(map[string]int{"services": 1, "envs": 1, "intents": 1}))

			ctx := context.Background()
			service, err := j.Store().GetService(ctx, "web", "nginx:latest")
			Expect(err).NotTo(HaveOccurred())
			Expect(service.Integration).To(Equal(&jed.Integration{Name: "frontend", Version: "v1.2.3"}))
			Expect(service.Network).To(Equal("backend"))
			env, err := j.Store().GetEnv(ctx, "web")
			Expect(err).NotTo(HaveOccurred())
			Expect(env.Vars).To(BeEmpty())
			intent, err := j.Store().GetIntent(ctx, "web")
			Expect(err).NotTo(HaveOccurred())
			Expect(intent.Replicas).To(Equal(2))
		})

		It("rejects import into a non-empty store", func() {
			ctx := context.Background()
			Expect(j.Store().SetEnv(ctx, jed.Env{Name: "web", Vars: map[string]string{}})).To(Succeed())

			res = doJSON(rtr, http.MethodPut, "/store/import", map[string]any{
				"schema":   jed.DBSchemaVersion,
				"services": []jed.Service{},
				"envs":     []jed.Env{},
				"intents":  []jed.Intent{},
			})
			Expect(res).To(HaveHTTPStatus(http.StatusConflict))
		})

		It("rejects schema mismatches", func() {
			res = doJSON(rtr, http.MethodPut, "/store/import", map[string]any{
				"schema":   "old",
				"services": []jed.Service{},
				"envs":     []jed.Env{},
				"intents":  []jed.Intent{},
			})
			Expect(res).To(HaveHTTPStatus(http.StatusUnprocessableEntity))
		})

		It("rejects intents without a matching imported service image", func() {
			res = doJSON(rtr, http.MethodPut, "/store/import", map[string]any{
				"schema":   jed.DBSchemaVersion,
				"services": []jed.Service{},
				"envs":     []jed.Env{},
				"intents":  []jed.Intent{{Name: "web", Image: "nginx:latest"}},
			})
			Expect(res).To(HaveHTTPStatus(http.StatusUnprocessableEntity))
		})

		It("rejects duplicate service keys", func() {
			service := jed.Service{Name: "web", Image: "nginx:latest", Network: "backend", Ports: map[string]string{}, Labels: map[string]string{}, Volumes: map[string]string{}}
			res = doJSON(rtr, http.MethodPut, "/store/import", map[string]any{
				"schema":   jed.DBSchemaVersion,
				"services": []jed.Service{service, service},
				"envs":     []jed.Env{},
				"intents":  []jed.Intent{},
			})
			Expect(res).To(HaveHTTPStatus(http.StatusUnprocessableEntity))
		})
	})

	Describe("service routes", func() {
		It("sets, gets, lists, and deletes services", func() {
			service := jed.Service{
				Integration: &jed.Integration{Name: "frontend", Version: "v1.2.3"},
				Image:       "nginx:latest",
				Ports:       map[string]string{},
				Labels:      map[string]string{},
				Volumes:     map[string]string{},
				Network:     "backend",
			}

			res = doJSON(rtr, http.MethodPut, "/store/services/web", service)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			res = doRequest(rtr, http.MethodGet, "/store/services/web?image=nginx:latest", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))
			var got jed.Service
			decodeJSON(res, &got)
			Expect(got.Name).To(Equal("web"))
			Expect(got.Integration).To(Equal(service.Integration))
			Expect(got.Image).To(Equal(service.Image))

			res = doRequest(rtr, http.MethodGet, "/store/services", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))
			var services []jed.Service
			decodeJSON(res, &services)
			Expect(services).To(ConsistOf(HaveField("Name", "web")))

			res = doRequest(rtr, http.MethodDelete, "/store/services/web?image=nginx:latest", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			res = doRequest(rtr, http.MethodGet, "/store/services/web?image=nginx:latest", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusNotFound))
		})

		It("rejects name mismatches", func() {
			service := jed.Service{Name: "api", Image: "nginx:latest", Network: "backend"}
			res = doJSON(rtr, http.MethodPut, "/store/services/web", service)
			Expect(res).To(HaveHTTPStatus(http.StatusUnprocessableEntity))
		})
	})

	Describe("env routes", func() {
		It("sets, gets, lists, and deletes envs", func() {
			env := jed.Env{Vars: map[string]string{"FOO": "bar"}}

			res = doJSON(rtr, http.MethodPut, "/store/envs/web", env)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			res = doRequest(rtr, http.MethodGet, "/store/envs/web", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))
			var got jed.Env
			decodeJSON(res, &got)
			Expect(got.Name).To(Equal("web"))
			Expect(got.Vars).To(HaveKeyWithValue("FOO", "bar"))

			res = doRequest(rtr, http.MethodGet, "/store/envs", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))
			var envs []jed.Env
			decodeJSON(res, &envs)
			Expect(envs).To(ConsistOf(HaveField("Name", "web")))

			res = doRequest(rtr, http.MethodDelete, "/store/envs/web", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			res = doRequest(rtr, http.MethodGet, "/store/envs/web", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))
			got = jed.Env{}
			decodeJSON(res, &got)
			Expect(got.Name).To(Equal("web"))
			Expect(got.Vars).To(BeEmpty())
		})

		It("rejects name mismatches", func() {
			env := jed.Env{Name: "api", Vars: map[string]string{"FOO": "bar"}}
			res = doJSON(rtr, http.MethodPut, "/store/envs/web", env)
			Expect(res).To(HaveHTTPStatus(http.StatusUnprocessableEntity))
		})
	})
})

func doJSON(handler http.Handler, method, target string, obj any) *httptest.ResponseRecorder {
	var body bytes.Buffer
	err := json.NewEncoder(&body).Encode(obj)
	Expect(err).NotTo(HaveOccurred())
	return doRequest(handler, method, target, &body)
}

func doRequest(handler http.Handler, method, target string, body *bytes.Buffer) *httptest.ResponseRecorder {
	var reader *bytes.Reader
	if body == nil {
		reader = bytes.NewReader(nil)
	} else {
		reader = bytes.NewReader(body.Bytes())
	}
	req := httptest.NewRequest(method, target, reader)
	res := httptest.NewRecorder()
	handler.ServeHTTP(res, req)
	return res
}

func decodeJSON(res *httptest.ResponseRecorder, out any) {
	err := json.NewDecoder(res.Body).Decode(out)
	Expect(err).NotTo(HaveOccurred())
}
