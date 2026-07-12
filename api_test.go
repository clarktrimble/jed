package jed_test

import (
	"bytes"
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

	Describe("service routes", func() {
		It("sets, gets, lists, and deletes services", func() {
			service := jed.Service{
				Image:   "nginx:latest",
				Ports:   map[string]string{},
				Labels:  map[string]string{},
				Volumes: map[string]string{},
				Network: "backend",
			}

			res = doJSON(rtr, http.MethodPut, "/store/services/web", service)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			res = doRequest(rtr, http.MethodGet, "/store/services/web", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))
			var got jed.Service
			decodeJSON(res, &got)
			Expect(got.Name).To(Equal("web"))
			Expect(got.Image).To(Equal(service.Image))

			res = doRequest(rtr, http.MethodGet, "/store/services", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))
			var services []jed.Service
			decodeJSON(res, &services)
			Expect(services).To(ConsistOf(HaveField("Name", "web")))

			res = doRequest(rtr, http.MethodDelete, "/store/services/web", nil)
			Expect(res).To(HaveHTTPStatus(http.StatusOK))

			res = doRequest(rtr, http.MethodGet, "/store/services/web", nil)
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
