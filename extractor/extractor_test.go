package extractor_test

import (
	"archive/tar"
	"bytes"
	"context"
	"io"
	"strings"
	"testing"

	"github.com/clarktrimble/jed/extractor"
	"github.com/pkg/errors"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestExtractor(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Extractor Suite")
}

var _ = Describe("Extractor", func() {
	var (
		ctx      context.Context
		client   *clientMock
		ext      *extractor.Extractor
		imageRef string
		paths    []string
		files    map[string][]byte
		err      error
	)

	BeforeEach(func() {
		ctx = context.Background()
		client = &clientMock{
			SendObjectFunc: func(ctx context.Context, method, path string, snd, rcv any) error {
				switch method {
				case "POST":
					Expect(path).To(HavePrefix("/containers/create?name=jed-extract-"))
					Expect(snd).To(Equal(map[string]string{"Image": "registry.example.com/app:v1"}))
					rcv.(*struct {
						Id string `json:"Id"`
					}).Id = "abc123"
				case "DELETE":
					Expect(path).To(Equal("/containers/abc123"))
				default:
					return errors.Errorf("unexpected object request %s %s", method, path)
				}
				return nil
			},
			SendJsonFunc: func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				Expect(body).To(BeNil())
				switch method + " " + path {
				case "POST /images/create?fromImage=registry.example.com%2Fapp&tag=v1":
					return []byte(`{"status":"Image is up to date"}`), nil
				case "GET /containers/abc123/archive?path=%2Fservice.yml":
					return tarFile("service.yml", []byte("name: app\n")), nil
				case "GET /containers/abc123/archive?path=%2Fservice.env":
					return tarFile("service.env", []byte("PORT=8080\n")), nil
				default:
					return nil, errors.Errorf("unexpected json request %s %s", method, path)
				}
			},
		}
		imageRef = "registry.example.com/app:v1"
		paths = []string{"/service.yml", "/service.env"}
		ext = extractor.New(client)
	})

	JustBeforeEach(func() {
		files, err = ext.Files(ctx, imageRef, paths...)
	})

	It("creates a stopped container, extracts files, and deletes the container", func() {
		Expect(err).ToNot(HaveOccurred())
		Expect(files).To(Equal(map[string][]byte{
			"/service.yml": []byte("name: app\n"),
			"/service.env": []byte("PORT=8080\n"),
		}))
		Expect(client.jsonCalls).To(Equal([]jsonCall{
			{method: "POST", path: "/images/create?fromImage=registry.example.com%2Fapp&tag=v1"},
			{method: "GET", path: "/containers/abc123/archive?path=%2Fservice.yml"},
			{method: "GET", path: "/containers/abc123/archive?path=%2Fservice.env"},
		}))
		Expect(client.objectCalls).To(HaveLen(2))
		Expect(client.objectCalls[0].method).To(Equal("POST"))
		Expect(client.objectCalls[1].method).To(Equal("DELETE"))
	})

	When("the image ref includes a registry port", func() {
		BeforeEach(func() {
			imageRef = "localhost:5000/app:v1"
			paths = []string{"/service.yml"}
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				switch method {
				case "POST":
					Expect(path).To(HavePrefix("/containers/create?name=jed-extract-"))
					Expect(snd).To(Equal(map[string]string{"Image": "localhost:5000/app:v1"}))
					rcv.(*struct {
						Id string `json:"Id"`
					}).Id = "abc123"
				case "DELETE":
					Expect(path).To(Equal("/containers/abc123"))
				default:
					return errors.Errorf("unexpected object request %s %s", method, path)
				}
				return nil
			}
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				Expect(body).To(BeNil())
				switch method + " " + path {
				case "POST /images/create?fromImage=localhost%3A5000%2Fapp&tag=v1":
					return []byte(`{"status":"Image is up to date"}`), nil
				case "GET /containers/abc123/archive?path=%2Fservice.yml":
					return tarFile("service.yml", []byte("name: app\n")), nil
				default:
					return nil, errors.Errorf("unexpected json request %s %s", method, path)
				}
			}
		})

		It("splits the tag after the image name", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(Equal(map[string][]byte{"/service.yml": []byte("name: app\n")}))
			Expect(client.jsonCalls[0]).To(Equal(jsonCall{method: "POST", path: "/images/create?fromImage=localhost%3A5000%2Fapp&tag=v1"}))
		})
	})

	When("no paths are requested", func() {
		BeforeEach(func() {
			paths = nil
		})

		It("returns an empty file map without calling Docker", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(Equal(map[string][]byte{}))
			Expect(client.jsonCalls).To(BeEmpty())
			Expect(client.objectCalls).To(BeEmpty())
		})
	})

	When("the image ref has no tag", func() {
		BeforeEach(func() {
			imageRef = "registry.example.com/app"
			paths = []string{"/service.yml"}
		})

		It("returns an error before pulling", func() {
			Expect(err).To(MatchError(`image ref "registry.example.com/app" has no tag`))
			Expect(client.jsonCalls).To(BeEmpty())
			Expect(client.objectCalls).To(BeEmpty())
		})
	})

	When("extracting a file fails", func() {
		BeforeEach(func() {
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				if method == "POST" {
					return nil, nil
				}
				return nil, errors.New("missing file")
			}
		})

		It("still deletes the temporary container", func() {
			Expect(err).To(MatchError("missing file; cleanup err: <nil>"))
			Expect(client.objectCalls).To(HaveLen(2))
			Expect(client.objectCalls[1].method).To(Equal("DELETE"))
		})
	})

	When("the archive has no regular file", func() {
		BeforeEach(func() {
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				if method == "POST" {
					return nil, nil
				}
				return tarDir("service.d"), nil
			}
		})

		It("returns an error", func() {
			Expect(err).To(HaveOccurred())
			Expect(err.Error()).To(ContainSubstring("archive contains no regular file"))
		})
	})
})

type objectCall struct {
	method string
	path   string
}

type jsonCall struct {
	method string
	path   string
}

type clientMock struct {
	SendObjectFunc func(ctx context.Context, method, path string, snd, rcv any) error
	SendJsonFunc   func(ctx context.Context, method, path string, body io.Reader) ([]byte, error)
	objectCalls    []objectCall
	jsonCalls      []jsonCall
}

func (c *clientMock) SendObject(ctx context.Context, method, path string, snd, rcv any) error {
	c.objectCalls = append(c.objectCalls, objectCall{method: method, path: path})
	return c.SendObjectFunc(ctx, method, path, snd, rcv)
}

func (c *clientMock) SendJson(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
	c.jsonCalls = append(c.jsonCalls, jsonCall{method: method, path: path})
	return c.SendJsonFunc(ctx, method, path, body)
}

func tarFile(name string, data []byte) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	Expect(tw.WriteHeader(&tar.Header{Name: name, Mode: 0644, Size: int64(len(data))})).To(Succeed())
	_, err := tw.Write(data)
	Expect(err).ToNot(HaveOccurred())
	Expect(tw.Close()).To(Succeed())
	return buf.Bytes()
}

func tarDir(name string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	Expect(tw.WriteHeader(&tar.Header{Name: strings.TrimSuffix(name, "/") + "/", Mode: 0755, Typeflag: tar.TypeDir})).To(Succeed())
	Expect(tw.Close()).To(Succeed())
	return buf.Bytes()
}
