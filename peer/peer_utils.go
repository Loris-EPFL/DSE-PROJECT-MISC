package peer

import (
	"sync"
	"time"
)

type DNSEntry struct {
	Domain     string
	IPAddress  string
	Expiration time.Time
}

// SafeMap is a thread-safe map with a generic key type K and value type V.
// The key type K must be comparable to satisfy the requirements of Go maps.
type SafeMap[K comparable, V any] struct {
	mu sync.RWMutex
	m  map[K]V
}

// NewSafeMap creates a new SafeMap instance.
func NewSafeMap[K comparable, V any]() SafeMap[K, V] {
	return SafeMap[K, V]{
		m: make(map[K]V),
	}
}

// Add inserts or updates an element in the map.
func (s *SafeMap[K, V]) Add(key K, value V) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.m[key] = value
}

// Remove deletes an element from the map.
func (s *SafeMap[K, V]) Remove(key K) {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.m, key)
}

// Get retrieves an element from the map.
func (s *SafeMap[K, V]) Get(key K) (V, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	val, exists := s.m[key]
	return val, exists
}

// Exists checks if a key exists in the map.
func (s *SafeMap[K, V]) Exists(key K) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	_, exists := s.m[key]
	return exists
}

// Size returns the number of elements in the map.
func (s *SafeMap[K, V]) Size() int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return len(s.m)
}

// Keys returns a slice of all keys in the map.
func (s *SafeMap[K, V]) Keys() []K {
	s.mu.RLock()
	defer s.mu.RUnlock()
	keys := make([]K, 0, len(s.m))
	for k := range s.m {
		keys = append(keys, k)
	}
	return keys
}

// ToMap returns a copy of the internal map.
func (s *SafeMap[K, V]) ToMap() map[K]V {
	s.mu.RLock()
	defer s.mu.RUnlock()
	copyMap := make(map[K]V, len(s.m))
	for k, v := range s.m {
		copyMap[k] = v
	}
	return copyMap
}
