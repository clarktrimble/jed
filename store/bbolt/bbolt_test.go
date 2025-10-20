package bbolt_test

import (
	"context"
	"os"
	"testing"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/clarktrimble/jed"
	"github.com/clarktrimble/jed/store"
	"github.com/clarktrimble/jed/store/bbolt"
)

func TestBboltStore(t *testing.T) {
	RegisterFailHandler(Fail)
	RunSpecs(t, "Bbolt Store Suite")
}

var _ = Describe("Bbolt Store", func() {
	var (
		currentStore *bbolt.Store
		dbPath       string
	)

	store.RunStoreContractTests(
		"Bbolt",
		func(ctx context.Context) (jed.Store, error) {
			// Create temp db file
			f, err := os.CreateTemp("", "bbolt-test-*.db")
			if err != nil {
				return nil, err
			}
			dbPath = f.Name()
			f.Close()

			currentStore, err = bbolt.New(dbPath)
			return currentStore, err
		},
		func() {
			if currentStore != nil {
				currentStore.Close()
			}
			os.Remove(dbPath)
		},
	)
})
