// Package bbolt provides a persistent implementation of jed.Store using BoltDB.
//
// Data is stored in a local BoltDB file for persistence across restarts.
package bbolt

import (
	"context"
	"encoding/json"

	"github.com/clarktrimble/jed"
	"github.com/pkg/errors"
	"go.etcd.io/bbolt"
)

var (
	servicesBucket = []byte("services")
	envsBucket     = []byte("envs")
)

// Todo: can store just handle name, bytes; unmarshalling into passed pointer?

// Store implements jed.Store interface on BoltDB.
type Store struct {
	db *bbolt.DB
}

// New creates a new bbolt-backed store.
func New(path string) (str *Store, err error) {

	db, err := bbolt.Open(path, 0600, nil)
	if err != nil {
		err = errors.Wrapf(err, "failed to open bbolt db")
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
		return nil
	})
	if err != nil {
		err = errors.Wrapf(err, "failed to create buckets")
		db.Close()
		return
	}

	str = &Store{db: db}
	return
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
