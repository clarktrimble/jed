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
	bolt "go.etcd.io/bbolt"
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

	It("stamps an empty database with the schema version", func() {
		f, err := os.CreateTemp("", "bbolt-schema-test-*.db")
		Expect(err).NotTo(HaveOccurred())
		dbPath := f.Name()
		Expect(f.Close()).To(Succeed())
		defer os.Remove(dbPath)

		currentStore, err = (&bbolt.Config{Path: dbPath}).New()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentStore.Close()).To(Succeed())
		currentStore = nil

		currentStore, err = (&bbolt.Config{Path: dbPath}).New()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentStore.Close()).To(Succeed())
		currentStore = nil
	})

	It("rejects legacy databases with data and no schema version", func() {
		f, err := os.CreateTemp("", "bbolt-schema-test-*.db")
		Expect(err).NotTo(HaveOccurred())
		dbPath := f.Name()
		Expect(f.Close()).To(Succeed())
		defer os.Remove(dbPath)

		currentStore, err = (&bbolt.Config{Path: dbPath, SkipSchemaCheck: true}).New()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentStore.SetService(context.Background(), jed.Service{Name: "web", Image: "nginx", Network: "svc-net"})).To(Succeed())
		Expect(currentStore.Close()).To(Succeed())
		currentStore = nil

		_, err = (&bbolt.Config{Path: dbPath}).New()
		Expect(err).To(MatchError(ContainSubstring("bbolt db has no schema version")))
	})

	It("rejects databases with an old schema version", func() {
		f, err := os.CreateTemp("", "bbolt-schema-test-*.db")
		Expect(err).NotTo(HaveOccurred())
		dbPath := f.Name()
		Expect(f.Close()).To(Succeed())
		defer os.Remove(dbPath)

		currentStore, err = (&bbolt.Config{Path: dbPath, SkipSchemaCheck: true}).New()
		Expect(err).NotTo(HaveOccurred())
		Expect(currentStore.SetService(context.Background(), jed.Service{Name: "web", Image: "nginx", Network: "svc-net"})).To(Succeed())
		Expect(currentStore.Close()).To(Succeed())
		currentStore = nil

		err = stampSchemaVersion(dbPath, "2")
		Expect(err).NotTo(HaveOccurred())

		_, err = (&bbolt.Config{Path: dbPath}).New()
		Expect(err).To(MatchError(ContainSubstring("bbolt db schema version \"2\" does not match current schema version \"3\"")))
	})

	It("times out instead of hanging when the database is locked", func() {
		f, err := os.CreateTemp("", "bbolt-lock-test-*.db")
		Expect(err).NotTo(HaveOccurred())
		dbPath := f.Name()
		Expect(f.Close()).To(Succeed())
		defer os.Remove(dbPath)

		lockedStore, err := (&bbolt.Config{Path: dbPath, SkipSchemaCheck: true}).New()
		Expect(err).NotTo(HaveOccurred())
		defer lockedStore.Close()

		_, err = (&bbolt.Config{Path: dbPath, Timeout: 10 * time.Millisecond, SkipSchemaCheck: true}).New()
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

			currentStore, err = (&bbolt.Config{Path: dbPath, SkipSchemaCheck: true}).New()
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

func stampSchemaVersion(path, version string) error {
	db, err := bolt.Open(path, 0600, nil)
	if err != nil {
		return err
	}
	defer db.Close()

	return db.Update(func(tx *bolt.Tx) error {
		bkt, err := tx.CreateBucketIfNotExists([]byte("meta"))
		if err != nil {
			return err
		}
		return bkt.Put([]byte("schema_version"), []byte(version))
	})
}
