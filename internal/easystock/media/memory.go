package media

import (
	"context"
	"sync"
	"time"
)

type object struct {
	contentType string
	body        []byte
}

// Memory is an in-process store. Tests upload with Put after reading the presigned key.
type Memory struct {
	mu      sync.Mutex
	objects map[string]object
}

func NewMemory() *Memory {
	return &Memory{objects: map[string]object{}}
}

func (m *Memory) PresignPut(_ context.Context, key, _ string, _ int64, _ time.Duration) (string, error) {
	return "memory://put/" + key, nil
}

func (m *Memory) PresignGet(_ context.Context, key string, _ time.Duration) (string, error) {
	return "memory://get/" + key, nil
}

// Put stores bytes the way a client would PUT them to the presigned URL.
func (m *Memory) Put(key, contentType string, body []byte) {
	m.mu.Lock()
	defer m.mu.Unlock()
	copied := append([]byte(nil), body...)
	m.objects[key] = object{contentType: contentType, body: copied}
}

func (m *Memory) Stat(_ context.Context, key string) (int64, string, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	obj, ok := m.objects[key]
	if !ok {
		return 0, "", ErrMissing
	}
	return int64(len(obj.body)), obj.contentType, nil
}
