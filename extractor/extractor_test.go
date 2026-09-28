package extractor_test

import (
	"archive/tar"
	"bytes"
	"context"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/clarktrimble/jed/extractor"
	"github.com/clarktrimble/jed/logger/loggertest"
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
		ext = extractor.New(client, loggertest.NewLoggerMock())
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

var _ = Describe("Extractor.LabelFiles", func() {
	var (
		ctx              context.Context
		client           *clientMock
		ext              *extractor.Extractor
		files            map[string][]byte
		err              error
		labels           map[string]string
		requestedLabels  []string
		requests         []string
		cleanupCancelled bool
	)

	BeforeEach(func() {
		ctx = context.Background()
		labels = map[string]string{"com.example.env": "/service.env"}
		requestedLabels = []string{"com.example.env"}
		requests = nil
		cleanupCancelled = false
		client = &clientMock{}
		client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
			switch method + " " + path {
			case "GET /images/registry.example.com%2Fapp:v1/json":
				return json.Unmarshal([]byte(`{"Id":"sha256:abc123","Config":{"Labels":`+mustJSON(labels)+`}}`), rcv)
			case "POST /containers/create?name=jed-extract-":
				return errors.New("container name must be checked by prefix")
			case "DELETE /containers/abc123":
				return nil
			}
			if method == "POST" && strings.HasPrefix(path, "/containers/create?name=jed-extract-") {
				Expect(snd).To(Equal(map[string]string{"Image": "sha256:abc123"}))
				rcv.(*struct {
					Id string `json:"Id"`
				}).Id = "abc123"
				return nil
			}
			return errors.Errorf("unexpected object request %s %s", method, path)
		}
		client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
			requests = append(requests, method+" "+path)
			switch method + " " + path {
			case "POST /images/create?fromImage=registry.example.com%2Fapp&tag=v1":
				return nil, nil
			case "GET /containers/abc123/archive?path=%2Fservice.env":
				return tarFile("service.env", []byte("PORT=8080\n")), nil
			default:
				return nil, errors.Errorf("unexpected json request %s %s", method, path)
			}
		}
		ext = extractor.New(client, loggertest.NewLoggerMock())
	})

	JustBeforeEach(func() {
		files, err = ext.LabelFiles(ctx, "registry.example.com/app:v1", requestedLabels...)
	})

	It("resolves a label and returns its file contents keyed by label", func() {
		Expect(err).ToNot(HaveOccurred())
		Expect(files).To(Equal(map[string][]byte{"com.example.env": []byte("PORT=8080\n")}))
		Expect(requests).To(Equal([]string{
			"POST /images/create?fromImage=registry.example.com%2Fapp&tag=v1",
			"GET /containers/abc123/archive?path=%2Fservice.env",
		}))
	})

	When("multiple labels name the same file", func() {
		BeforeEach(func() {
			labels["com.example.copy"] = "/service.env"
			requestedLabels = []string{"com.example.env", "com.example.copy"}
		})

		It("extracts the file once and returns it for each label", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(Equal(map[string][]byte{
				"com.example.env":  []byte("PORT=8080\n"),
				"com.example.copy": []byte("PORT=8080\n"),
			}))
			Expect(requests).To(HaveLen(2))
		})
	})

	When("labels are missing or empty", func() {
		BeforeEach(func() {
			labels = map[string]string{"com.example.empty": ""}
			requestedLabels = []string{"com.example.missing", "com.example.empty"}
		})

		It("omits them without creating a container", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(Equal(map[string][]byte{}))
			Expect(client.objectCalls).To(HaveLen(1))
			Expect(requests).To(Equal([]string{"POST /images/create?fromImage=registry.example.com%2Fapp&tag=v1"}))
		})
	})

	When("a declared file is empty", func() {
		BeforeEach(func() {
			labels["com.example.absent"] = ""
			requestedLabels = []string{"com.example.env", "com.example.absent"}
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				requests = append(requests, method+" "+path)
				if method == "POST" {
					return nil, nil
				}
				return tarFile("service.env", nil), nil
			}
		})

		It("keeps an entry for the empty file but omits the empty label", func() {
			Expect(err).ToNot(HaveOccurred())
			Expect(files).To(HaveKey("com.example.env"))
			Expect(files["com.example.env"]).To(BeEmpty())
			Expect(files).ToNot(HaveKey("com.example.absent"))
		})
	})

	When("inspection fails", func() {
		BeforeEach(func() {
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				if method == "GET" {
					return errors.New("inspection failed")
				}
				return errors.New("unexpected request")
			}
		})

		It("returns the error without creating a container", func() {
			Expect(err).To(MatchError("inspection failed"))
			Expect(client.objectCalls).To(HaveLen(1))
		})
	})

	When("the extraction context is cancelled", func() {
		BeforeEach(func() {
			var cancel context.CancelFunc
			ctx, cancel = context.WithCancel(ctx)
			sendObject := client.SendObjectFunc
			client.SendObjectFunc = func(ctx context.Context, method, path string, snd, rcv any) error {
				if method == "DELETE" {
					cleanupCancelled = ctx.Err() != nil
				}
				return sendObject(ctx, method, path, snd, rcv)
			}
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				if method == "POST" {
					return nil, nil
				}
				cancel()
				return nil, context.Canceled
			}
		})

		It("returns cancellation and still uses a live context for cleanup", func() {
			Expect(err).To(MatchError("context canceled; cleanup err: <nil>"))
			Expect(cleanupCancelled).To(BeFalse())
			Expect(client.objectCalls).To(HaveLen(3))
		})
	})

	When("a declared file cannot be extracted", func() {
		BeforeEach(func() {
			client.SendJsonFunc = func(ctx context.Context, method, path string, body io.Reader) ([]byte, error) {
				if method == "POST" {
					return nil, nil
				}
				return nil, errors.New("missing file")
			}
		})

		It("returns the extraction error and cleans up the container", func() {
			Expect(err).To(MatchError("missing file; cleanup err: <nil>"))
			Expect(client.objectCalls).To(HaveLen(3))
			Expect(client.objectCalls[2]).To(Equal(objectCall{method: "DELETE", path: "/containers/abc123"}))
		})
	})
})

func mustJSON(value any) string {
	data, err := json.Marshal(value)
	Expect(err).ToNot(HaveOccurred())
	return string(data)
}

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
