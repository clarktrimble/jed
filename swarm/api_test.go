package swarm_test

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("API", func() {
	var (
		client *ClientMock
		sw     *swarm.Swarm
		rtr    *http.ServeMux
		res    *httptest.ResponseRecorder
	)

	BeforeEach(func() {
		client = &ClientMock{}
		sw = swarm.New(client, nopLogger{})
		rtr = http.NewServeMux()
		sw.Register(rtr)
	})

	It("lists secrets", func() {
		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			Expect(method).To(Equal(http.MethodGet))
			Expect(path).To(Equal("/v1.52/secrets"))
			mockResponse([]secretItem{{ID: "sec-123", Spec: specName{Name: "db_password_v1"}}}, rcv)
			return nil
		}

		res = doRequest(rtr, http.MethodGet, "/swarm/secrets", nil)
		Expect(res).To(HaveHTTPStatus(http.StatusOK))

		var got []swarm.SecretResource
		decodeJSON(res, &got)
		Expect(got).To(Equal([]swarm.SecretResource{{ID: "sec-123", Name: "db_password_v1"}}))
	})

	It("creates secrets from the request body", func() {
		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			switch path {
			case "/v1.52/secrets":
				mockResponse([]secretItem{}, rcv)
			case "/v1.52/secrets/create":
				mockResponse(map[string]string{"ID": "sec-new"}, rcv)
			default:
				Fail("unexpected path: " + path)
			}
			return nil
		}

		res = doRequest(rtr, http.MethodPost, "/swarm/secrets/db_password", bytes.NewBufferString("hunter2"))
		Expect(res).To(HaveHTTPStatus(http.StatusOK))

		var got map[string]string
		decodeJSON(res, &got)
		Expect(got).To(HaveKeyWithValue("ID", "sec-new"))

		call := findCall(client.SendObjectCalls(), http.MethodPost, "/v1.52/secrets/create")
		Expect(call).NotTo(BeNil())
		Expect(sentData(call.Snd)).To(HaveKeyWithValue("Data", "aHVudGVyMg=="))
	})

	It("lists configs", func() {
		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			Expect(method).To(Equal(http.MethodGet))
			Expect(path).To(Equal("/v1.52/configs"))
			mockResponse([]secretItem{{ID: "cfg-123", Spec: specName{Name: "app_config_v1", Data: []byte("setting: true")}}}, rcv)
			return nil
		}

		res = doRequest(rtr, http.MethodGet, "/swarm/configs", nil)
		Expect(res).To(HaveHTTPStatus(http.StatusOK))

		var got []swarm.ConfigResource
		decodeJSON(res, &got)
		Expect(got).To(Equal([]swarm.ConfigResource{{ID: "cfg-123", Name: "app_config_v1", Data: []byte("setting: true")}}))
	})

	It("gets the latest config by base name", func() {
		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			Expect(method).To(Equal(http.MethodGet))
			Expect(path).To(Equal("/v1.52/configs"))
			mockResponse([]secretItem{
				{ID: "cfg-1", Spec: specName{Name: "app_config_v1", Data: []byte("old")}},
				{ID: "cfg-2", Spec: specName{Name: "app_config_v2", Data: []byte("new")}},
			}, rcv)
			return nil
		}

		res = doRequest(rtr, http.MethodGet, "/swarm/configs/app_config", nil)
		Expect(res).To(HaveHTTPStatus(http.StatusOK))

		var got swarm.ConfigResource
		decodeJSON(res, &got)
		Expect(got).To(Equal(swarm.ConfigResource{ID: "cfg-2", Name: "app_config_v2", Data: []byte("new")}))
	})

	It("returns not found for a missing latest config", func() {
		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			Expect(method).To(Equal(http.MethodGet))
			Expect(path).To(Equal("/v1.52/configs"))
			mockResponse([]secretItem{}, rcv)
			return nil
		}

		res = doRequest(rtr, http.MethodGet, "/swarm/configs/app_config", nil)
		Expect(res).To(HaveHTTPStatus(http.StatusNotFound))
	})

	It("creates configs from the request body", func() {
		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			switch path {
			case "/v1.52/configs":
				mockResponse([]secretItem{}, rcv)
			case "/v1.52/configs/create":
				mockResponse(map[string]string{"ID": "cfg-new"}, rcv)
			default:
				Fail("unexpected path: " + path)
			}
			return nil
		}

		res = doRequest(rtr, http.MethodPost, "/swarm/configs/app_config", bytes.NewBufferString("setting: true"))
		Expect(res).To(HaveHTTPStatus(http.StatusOK))

		var got map[string]string
		decodeJSON(res, &got)
		Expect(got).To(HaveKeyWithValue("ID", "cfg-new"))

		call := findCall(client.SendObjectCalls(), http.MethodPost, "/v1.52/configs/create")
		Expect(call).NotTo(BeNil())
		Expect(sentData(call.Snd)).To(HaveKeyWithValue("Data", "c2V0dGluZzogdHJ1ZQ=="))
	})

	It("deletes configs", func() {
		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			Expect(method).To(Equal(http.MethodDelete))
			Expect(path).To(Equal("/v1.52/configs/cfg-123"))
			return nil
		}

		res = doRequest(rtr, http.MethodDelete, "/swarm/configs/by_id/cfg-123", nil)
		Expect(res).To(HaveHTTPStatus(http.StatusOK))
	})
})

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

func decodeJSON(res *httptest.ResponseRecorder, dest any) {
	Expect(json.Unmarshal(res.Body.Bytes(), dest)).To(Succeed())
}

func sentData(obj any) map[string]string {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	var got map[string]string
	Expect(json.Unmarshal(data, &got)).To(Succeed())
	return got
}
