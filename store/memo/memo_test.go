package memo_test

import (
	"context"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/store"
	"github.com/clarktrimble/jed/store/memo"
)

func TestMemoStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Memo Store Suite")
}

var _ = Describe("Memo Store", func() {
	store.RunStoreContractTests(
		"Memo",
		func(ctx context.Context) (jed.Store, error) {
			return memo.New(), nil
		},
		func() {
			// No cleanup needed - in-memory storage will be garbage collected
		},
	)
})
