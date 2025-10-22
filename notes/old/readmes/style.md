# Style Guide

## Golang

Function style with named returns and naked return:

```go
func (str *Store) GetService(ctx context.Context, name string) (service jed.Service, err error) {

	err = str.db.View(func(tx *bbolt.Tx) error {
		bkt := tx.Bucket(servicesBucket)
		data := bkt.Get([]byte(name))
		if data == nil {
			return errors.Errorf("service not found: %s", name)
		}
		return json.Unmarshal(data, &service)
	})

	return
}
```

Preferences:

1. Named params / Naked returns - `(service jed.Service, err error)` unless unnamed is clearer/safer
2. Blank line after function signature before logic, unless it's a one or two-liner
3. Short var names - `bkt` instead of `b` or `bucket`
4. `errors.Errorf` (from pkg/errors) for new error messages
5. Wrap library errors with `errors.Wrapf` to add context. Don't wrap errors already from our functions (avoid double-wrapping).
6. No inline error checking - avoid `if err := foo(); err != nil`, separate the call and check
7. `errors.Wrapf` returns nil if err is nil, so no need for `if err != nil` before wrapping
8. No blank line between wrapping and return when wrapping potentially nil errors
9. Prefer readable code to comments.  Goal is to omit obvious comments.

## Markdown

1. Avoid bold formatting - plain text is clearer
