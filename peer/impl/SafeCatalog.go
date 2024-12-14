package impl

import (
	"sync"

	"go.dedis.ch/cs438/peer"
)

type SafeCatalog struct {
	mu      sync.RWMutex
	catalog peer.Catalog
}

func NewSafeCatalog() SafeCatalog {
	return SafeCatalog{
		catalog: make(peer.Catalog),
	}
}

func (sc *SafeCatalog) Update(key string, peerAddr string, self string) {
	if peerAddr == self {
		return
	}

	sc.mu.Lock()
	defer sc.mu.Unlock()

	peersWithKey, exists := sc.catalog[key]

	if !exists {
		peersWithKey = make(map[string]struct{})
		sc.catalog[key] = peersWithKey
	}
	peersWithKey[peerAddr] = struct{}{}
}

func (sc *SafeCatalog) Get() peer.Catalog {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	catalogCopy := make(peer.Catalog)

	for key, peers := range sc.catalog {
		peersCopy := make(map[string]struct{})
		for peerAddr := range peers {
			peersCopy[peerAddr] = struct{}{}
		}
		catalogCopy[key] = peersCopy
	}
	return catalogCopy
}

func (sc *SafeCatalog) RemoveKey(key string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	delete(sc.catalog, key)
}

func (sc *SafeCatalog) Remove(key, peer string) {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	delete(sc.catalog[key], peer)
}

func (sc *SafeCatalog) GetPeersForKey(key string) map[string]struct{} {
	sc.mu.Lock()
	defer sc.mu.Unlock()

	peersCopy := make(map[string]struct{})

	if peers, exists := sc.catalog[key]; exists {

		for peerAddr := range peers {
			peersCopy[peerAddr] = struct{}{}
		}
	}
	return peersCopy
}
