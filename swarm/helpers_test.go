package swarm_test

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

func TestSwarm(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Swarm Suite")
}

// nopLogger is a no-op logger for tests.
type nopLogger struct{}

func (nopLogger) Info(ctx context.Context, msg string, kv ...any)             {}
func (nopLogger) Debug(ctx context.Context, msg string, kv ...any)            {}
func (nopLogger) Trace(ctx context.Context, msg string, kv ...any)            {}
func (nopLogger) Error(ctx context.Context, msg string, err error, kv ...any) {}
func (nopLogger) WithFields(ctx context.Context, kv ...any) context.Context   { return ctx }
func (nopLogger) SetLevel(ctx context.Context, level string) error            { return nil }
func (nopLogger) GetLevel() string                                            { return "" }

func loadTestData(name string) []byte {
	data, err := os.ReadFile("../test/data/swarm/" + name)
	Expect(err).NotTo(HaveOccurred())
	return data
}

type secretItem struct {
	ID   string   `json:"ID"`
	Spec specName `json:"Spec"`
}

type specName struct {
	Name string `json:"Name"`
	Data []byte `json:"Data,omitempty"`
}

func mockResponse(obj, rcv any) {
	data, err := json.Marshal(obj)
	Expect(err).NotTo(HaveOccurred())
	err = json.Unmarshal(data, rcv)
	Expect(err).NotTo(HaveOccurred())
}

func findCall(calls []struct {
	Ctx    context.Context
	Method string
	Path   string
	Snd    any
	Rcv    any
}, method, pathSubstring string) *struct {
	Ctx    context.Context
	Method string
	Path   string
	Snd    any
	Rcv    any
} {
	for i := range calls {
		if calls[i].Method == method && strings.Contains(calls[i].Path, pathSubstring) {
			return &calls[i]
		}
	}
	return nil
}
