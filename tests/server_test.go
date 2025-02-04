package tests

import (
	"testing"
)

type TestServer struct {
	data map[string]string
}

func NewTestServer() *TestServer {
	return &TestServer{data: make(map[string]string)}
}

func (s *TestServer) Set(key, value string) error {
	s.data[key] = value
	return nil
}

func (s *TestServer) Get(key string) (string, bool) {
	val, exists := s.data[key]
	return val, exists
}

func (s *TestServer) Delete(key string) error {
	delete(s.data, key)
	return nil
}

func (s *TestServer) Keys() []string {
	var keys []string
	for k := range s.data {
		keys = append(keys, k)
	}
	return keys
}

func TestServer_SetAndGet(t *testing.T) {
	s := NewTestServer()
	if err := s.Set("username", "johndoe"); err != nil {
		t.Fatalf("failed to set key: %v", err)
	}
	val, ok := s.Get("username")
	if !ok || val != "johndoe" {
		t.Errorf("expected value 'johndoe', got %q", val)
	}
}

func TestServer_DeleteKey(t *testing.T) {
	s := NewTestServer()
	if err := s.Set("username", "johndoe"); err != nil {
		t.Fatalf("failed to set key: %v", err)
	}
	if err := s.Delete("username"); err != nil {
		t.Fatalf("failed to delete key: %v", err)
	}
	_, ok := s.Get("username")
	if ok {
		t.Errorf("expected key 'username' to be deleted")
	}
}

func TestServer_KeysListing(t *testing.T) {
	s := NewTestServer()
	keysToAdd := map[string]string{
		"username": "johndoe",
		"email":    "john@example.com",
		"age":      "30",
	}
	for k, v := range keysToAdd {
		if err := s.Set(k, v); err != nil {
			t.Fatalf("failed to set key %q: %v", k, err)
		}
	}
	keys := s.Keys()
	if len(keys) != len(keysToAdd) {
		t.Errorf("expected %d keys, got %d", len(keysToAdd), len(keys))
	}
}
