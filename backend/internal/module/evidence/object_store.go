package evidence

import (
	"bytes"
	"context"
	"errors"
	"io"
	"sync"
)

var ErrObjectNotFound = errors.New("evidence object not found")

type ObjectStore interface {
	Put(ctx context.Context, key, contentType string, body []byte) error
	Get(ctx context.Context, key string) (Object, error)
	Delete(ctx context.Context, key string) error
}

type MemoryObjectStore struct {
	mu      sync.RWMutex
	objects map[string]memoryObject
}

type memoryObject struct {
	contentType string
	body        []byte
}

func NewMemoryObjectStore() *MemoryObjectStore {
	return &MemoryObjectStore{objects: make(map[string]memoryObject)}
}

func (s *MemoryObjectStore) Put(_ context.Context, key, contentType string, body []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.objects[key] = memoryObject{contentType: contentType, body: append([]byte(nil), body...)}
	return nil
}

func (s *MemoryObjectStore) Get(_ context.Context, key string) (Object, error) {
	s.mu.RLock()
	item, ok := s.objects[key]
	s.mu.RUnlock()
	if !ok {
		return Object{}, ErrObjectNotFound
	}
	return Object{
		Body:        io.NopCloser(bytes.NewReader(item.body)),
		ContentType: item.contentType, Size: int64(len(item.body)),
	}, nil
}

func (s *MemoryObjectStore) Delete(_ context.Context, key string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.objects, key)
	return nil
}
