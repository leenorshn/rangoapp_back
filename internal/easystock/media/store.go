package media

import (
	"context"
	"errors"
	"time"
)

// ErrMissing means the object key has no bytes yet.
var ErrMissing = errors.New("object missing")

// Store is S3-compatible object storage. Keys are chosen by the server.
type Store interface {
	PresignPut(ctx context.Context, key, contentType string, size int64, expires time.Duration) (string, error)
	PresignGet(ctx context.Context, key string, expires time.Duration) (string, error)
	Stat(ctx context.Context, key string) (size int64, contentType string, err error)
}
