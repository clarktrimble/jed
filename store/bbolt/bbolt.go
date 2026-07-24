// Package bbolt provides a persistent implementation of jed.Store using BoltDB.
//
// Data is stored in a local BoltDB file for persistence across restarts.
package bbolt

import (
	"context"
	"encoding/json"
	"time"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
	bberrors "go.etcd.io/bbolt/errors"
)

const (
	// DefaultDBPath is the conventional path for production Jed data.
	DefaultDBPath = "/data/jed.db"
)

// DefaultOpenTimeout is how long New waits for the BoltDB file lock.
var DefaultOpenTimeout = 2 * time.Second

var (
	servicesBucket   = []byte("services")
	envsBucket       = []byte("envs")
	intentsBucket    = []byte("intents")
	metaBucket       = []byte("meta")
	schemaVersionKey = []byte("schema_version")
)

// Todo: can store just handle name, bytes; unmarshalling into passed pointer?

// Config controls bbolt store creation.
type Config struct {
	// Timeout is the amount of time to wait for the database file lock.
	Timeout time.Duration `json:"timeout" default:"2s"`

	// Path is the path to the bbolt database.
	Path string `json:"path" default:"/data/jed.db"`

	// SkipSchemaCheck disables reading, writing, and validating DB schema version.
	SkipSchemaCheck bool `json:"skip_schema_check" default:"false"`
}

// Store implements jed.Store interface on BoltDB.
type Store struct {
	db *bbolt.DB
}

// New creates a new bbolt-backed store using cfg.Path.
func (cfg *Config) New() (str *Store, err error) {
	if cfg == nil {
		return nil, errors.New("bbolt config is nil")
	}

	path := DefaultDBPath
	if cfg.Path != "" {
		path = cfg.Path
	}

	timeout := DefaultOpenTimeout
	if cfg.Timeout != 0 {
		timeout = cfg.Timeout
	}

	return open(path, timeout, cfg.SkipSchemaCheck)
}

func open(path string, timeout time.Duration, skipSchemaCheck bool) (str *Store, err error) {
	db, err := bbolt.Open(path, 0600, &bbolt.Options{Timeout: timeout})
	if err != nil {
		if errors.Is(err, bberrors.ErrTimeout) {
			err = errors.Wrapf(err, "failed to open bbolt db %q: timed out waiting for file lock after %s; another process has a lock?", path, timeout)
			return
		}
		err = errors.Wrapf(err, "failed to open bbolt db %q", path)
		return
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucketIfNotExists(servicesBucket)
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(envsBucket)
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(intentsBucket)
		if err != nil {
			return err
		}
		_, err = tx.CreateBucketIfNotExists(metaBucket)
		if err != nil {
			return err
		}
		if skipSchemaCheck {
			return nil
		}
		return checkSchemaVersion(tx)
	})
	if err != nil {
		err = errors.Wrapf(err, "failed to initialize bbolt db")
		db.Close()
		return
	}

	str = &Store{db: db}
	return
}

func checkSchemaVersion(tx *bbolt.Tx) error {
	meta := tx.Bucket(metaBucket)
	data := meta.Get(schemaVersionKey)
	if data == nil {
		if !storeIsEmpty(tx) {
			return errors.New("bbolt db has no schema version; open with skip_schema_check to inspect or migrate")
		}
		return meta.Put(schemaVersionKey, []byte(jed.DBSchemaVersion))
	}

	stored := string(data)
	if stored != jed.DBSchemaVersion {
		return errors.Errorf("bbolt db schema version %q does not match current schema version %q", stored, jed.DBSchemaVersion)
	}
	return nil
}

func storeIsEmpty(tx *bbolt.Tx) bool {
	return tx.Bucket(servicesBucket).Stats().KeyN == 0 && tx.Bucket(envsBucket).Stats().KeyN == 0 && tx.Bucket(intentsBucket).Stats().KeyN == 0
}

// Close closes the underlying database.
func (str *Store) Close() error {
	return str.db.Close()
}

// GetService retrieves a service definition by name.
func (str *Store) GetService(ctx context.Context, name string) (service jed.Service, err error) {

	err = str.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(servicesBucket)
		data := bkt.Get([]byte(name))
		if data == nil {
			return jed.NotFoundError{Kind: "service", Name: name}
		}
		err := json.Unmarshal(data, &service)
		err = errors.Wrapf(err, "failed to decode service")
		return err
	})
	return
}

// SetService creates or updates a service definition.
func (str *Store) SetService(ctx context.Context, service jed.Service) (err error) {

	data, err := json.Marshal(service)
	if err != nil {
		err = errors.Wrapf(err, "failed to encode service")
		return
	}

	err = str.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(servicesBucket)
		return bkt.Put([]byte(service.Name), data)
	})

	return
}

// DelService removes a service definition by name.
func (str *Store) DelService(ctx context.Context, name string) (err error) {
	// Todo: check key existence before delete, bbolt is silent on missing keys.

	err = str.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(servicesBucket)
		return bkt.Delete([]byte(name))
	})

	return
}

// Services returns all service definitions.
func (str *Store) Services(ctx context.Context) (services []jed.Service, err error) {

	err = str.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(servicesBucket)
		return bkt.ForEach(func(key, val []byte) error {
			var svc jed.Service
			err := json.Unmarshal(val, &svc)
			if err != nil {
				return errors.Wrapf(err, "failed to decode service")
			}
			services = append(services, svc)
			return nil
		})
	})
	err = errors.Wrapf(err, "failed to list services")
	return
}

// GetEnv retrieves environment variables by name.
func (str *Store) GetEnv(ctx context.Context, name string) (env jed.Env, err error) {

	err = str.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(envsBucket)
		data := bkt.Get([]byte(name))
		if data == nil {
			// Return empty env when not found (not an error condition)
			env = jed.Env{Name: name, Vars: make(map[string]string)}
			return nil
		}
		err := json.Unmarshal(data, &env)
		err = errors.Wrapf(err, "failed to decode env")
		return err
	})
	err = errors.Wrapf(err, "failed to get env")
	return
}

// SetEnv creates or updates environment variables.
func (str *Store) SetEnv(ctx context.Context, env jed.Env) (err error) {

	data, err := json.Marshal(env)
	if err != nil {
		err = errors.Wrapf(err, "failed to encode env")
		return
	}

	err = str.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(envsBucket)
		return bkt.Put([]byte(env.Name), data)
	})
	err = errors.Wrapf(err, "failed to update env")
	return
}

// DelEnv removes environment variables by name.
func (str *Store) DelEnv(ctx context.Context, name string) (err error) {
	// Todo: check key existence before delete, bbolt is silent on missing keys.

	err = str.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(envsBucket)
		return bkt.Delete([]byte(name))
	})
	err = errors.Wrapf(err, "failed to delete env")
	return
}

// Envs returns all environment variable sets.
func (str *Store) Envs(ctx context.Context) (envs []jed.Env, err error) {

	err = str.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(envsBucket)
		return bkt.ForEach(func(key, val []byte) error {
			var env jed.Env
			err := json.Unmarshal(val, &env)
			if err != nil {
				return errors.Wrapf(err, "failed to decode env")
			}
			envs = append(envs, env)
			return nil
		})
	})
	err = errors.Wrapf(err, "failed to list envs")
	return
}

// GetIntent retrieves the desired active image and replica count by service name.
func (str *Store) GetIntent(ctx context.Context, name string) (intent jed.Intent, err error) {

	err = str.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(intentsBucket)
		data := bkt.Get([]byte(name))
		if data == nil {
			return jed.NotFoundError{Kind: "intent", Name: name}
		}
		err := json.Unmarshal(data, &intent)
		err = errors.Wrapf(err, "failed to decode intent")
		return err
	})
	return
}

// SetIntent creates or updates the desired active image and replica count.
func (str *Store) SetIntent(ctx context.Context, intent jed.Intent) (err error) {

	data, err := json.Marshal(intent)
	if err != nil {
		err = errors.Wrapf(err, "failed to encode intent")
		return
	}

	err = str.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(intentsBucket)
		return bkt.Put([]byte(intent.Name), data)
	})

	return
}

// DelIntent removes the desired active image and replica count by service name.
func (str *Store) DelIntent(ctx context.Context, name string) (err error) {

	err = str.db.Update(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(intentsBucket)
		return bkt.Delete([]byte(name))
	})

	return
}

// Intents returns all desired active images and replica counts.
func (str *Store) Intents(ctx context.Context) (intents []jed.Intent, err error) {

	err = str.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(intentsBucket)
		return bkt.ForEach(func(key, val []byte) error {
			var intent jed.Intent
			err := json.Unmarshal(val, &intent)
			if err != nil {
				return errors.Wrapf(err, "failed to decode intent")
			}
			intents = append(intents, intent)
			return nil
		})
	})
	err = errors.Wrapf(err, "failed to list intents")
	return
}
