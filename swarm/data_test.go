package swarm_test

import (
	"context"
	"encoding/json"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed/swarm"
)

var _ = Describe("Data", func() {
	var (
		client  *ClientMock
		sw      *swarm.Swarm
		ctx     context.Context
		err     error
		secrets []swarm.SecretResource
		configs []swarm.ConfigResource
		id      string
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &ClientMock{}
		sw = swarm.New(client, nopLogger{})
	})

	Describe("ListSecrets", func() {
		BeforeEach(func() {
			testData := loadTestData("get-scrts.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				return json.Unmarshal(testData, rcv)
			}
		})

		JustBeforeEach(func() {
			secrets, err = sw.ListSecrets(ctx)
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should parse secrets", func() {
			Expect(secrets).To(HaveLen(6))
			Expect(secrets[0].ID).To(Equal("53o7ekx6p3c5q22lzqgf3wv09"))
			Expect(secrets[0].Name).To(Equal("s3_secret_key_v1"))
		})
	})

	Describe("ListConfigs", func() {
		BeforeEach(func() {
			testData := loadTestData("get-configs.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				return json.Unmarshal(testData, rcv)
			}
		})

		JustBeforeEach(func() {
			configs, err = sw.ListConfigs(ctx)
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should handle empty list", func() {
			Expect(configs).To(HaveLen(0))
		})
	})

	Describe("CreateSecret", func() {
		var (
			sentName string
			sentData string
		)

		BeforeEach(func() {
			testData := loadTestData("get-scrts.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				if method == "GET" && strings.Contains(path, "/secrets") {
					return json.Unmarshal(testData, rcv)
				}
				if method == "POST" && strings.Contains(path, "/secrets/create") {
					data, _ := json.Marshal(snd)
					var req map[string]string
					Expect(json.Unmarshal(data, &req)).To(Succeed())
					sentName = req["Name"]
					sentData = req["Data"]
					mockResponse(map[string]string{"ID": "new-secret-id"}, rcv)
				}
				return nil
			}
		})

		JustBeforeEach(func() {
			id, err = sw.CreateSecret(ctx, "bfc_api_key", []byte("supersecret"))
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the new ID", func() {
			Expect(id).To(Equal("new-secret-id"))
		})

		It("should use next version number", func() {
			Expect(sentName).To(Equal("bfc_api_key_v4"))
		})

		It("should base64 encode the data", func() {
			Expect(sentData).To(Equal("c3VwZXJzZWNyZXQ="))
		})
	})

	Describe("CreateConfig", func() {
		var sentName string

		BeforeEach(func() {
			testData := loadTestData("get-configs.json")
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				if method == "GET" && strings.Contains(path, "/configs") {
					return json.Unmarshal(testData, rcv)
				}
				if method == "POST" && strings.Contains(path, "/configs/create") {
					data, _ := json.Marshal(snd)
					var req map[string]string
					Expect(json.Unmarshal(data, &req)).To(Succeed())
					sentName = req["Name"]
					mockResponse(map[string]string{"ID": "new-config-id"}, rcv)
				}
				return nil
			}
		})

		JustBeforeEach(func() {
			id, err = sw.CreateConfig(ctx, "my_config", []byte("config data"))
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should return the new ID", func() {
			Expect(id).To(Equal("new-config-id"))
		})

		It("should start at v1 when no existing versions", func() {
			Expect(sentName).To(Equal("my_config_v1"))
		})
	})

	Describe("DeleteConfig", func() {
		var calledPath string

		BeforeEach(func() {
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				calledPath = path
				return nil
			}
		})

		JustBeforeEach(func() {
			err = sw.DeleteConfig(ctx, "config-id-123")
		})

		It("should return without error", func() {
			Expect(err).NotTo(HaveOccurred())
		})

		It("should call DELETE on correct path", func() {
			Expect(calledPath).To(Equal("/v1.52/configs/config-id-123"))
		})
	})
})
