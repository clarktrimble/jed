package bbolt_test

import (
	"context"
	"os"
	"testing"
	"time"

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

	It("returns an error when config is nil", func() {
		_, err := (*bbolt.Config)(nil).New()
		Expect(err).To(MatchError("bbolt config is nil"))
	})

	It("times out instead of hanging when the database is locked", func() {
		f, err := os.CreateTemp("", "bbolt-lock-test-*.db")
		Expect(err).NotTo(HaveOccurred())
		dbPath := f.Name()
		Expect(f.Close()).To(Succeed())
		defer os.Remove(dbPath)

		lockedStore, err := (&bbolt.Config{Path: dbPath}).New()
		Expect(err).NotTo(HaveOccurred())
		defer lockedStore.Close()

		_, err = (&bbolt.Config{Path: dbPath, Timeout: 10 * time.Millisecond}).New()
		Expect(err).To(MatchError(ContainSubstring("timed out waiting for file lock")))
		Expect(err).To(MatchError(ContainSubstring("another process has a lock?")))
	})

	store.RunStoreContractTests(
		"Bbolt",
		func(ctx context.Context) (jed.Store, error) {
			f, err := os.CreateTemp("", "bbolt-test-*.db")
			if err != nil {
				return nil, err
			}
			dbPath = f.Name()
			f.Close()

			currentStore, err = (&bbolt.Config{Path: dbPath}).New()
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
