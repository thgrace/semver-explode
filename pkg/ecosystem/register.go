package ecosystem

import (
	"fmt"
	"sort"
	"sync"
)

// entry holds a constructor and the lazily-initialized singleton instance.
type entry struct {
	ctor func() Ecosystem
	once sync.Once
	inst Ecosystem
}

var (
	registryMu sync.RWMutex
	entries    = map[string]*entry{}
)

// Register associates name with a constructor for that Ecosystem. Intended
// to be called from each pkg/<eco> package's init(). Panics on duplicate
// registration — that's a programmer error worth surfacing loudly.
func Register(name string, ctor func() Ecosystem) {
	registryMu.Lock()
	defer registryMu.Unlock()
	if _, dup := entries[name]; dup {
		panic(fmt.Sprintf("ecosystem: %q already registered", name))
	}
	entries[name] = &entry{ctor: ctor}
}

// Lookup returns the shared Ecosystem singleton for name. The bool is false
// when the name is unknown. The first call for a given name runs the
// registered constructor; all subsequent calls return the same instance.
//
// The returned Ecosystem is shared across all callers and must be safe to
// use concurrently. Both *Npm and *Pypi are stateless beyond holding a
// Registry pointer, which is itself safe for concurrent use.
func Lookup(name string) (Ecosystem, bool) {
	registryMu.RLock()
	e, ok := entries[name]
	registryMu.RUnlock()
	if !ok {
		return nil, false
	}
	e.once.Do(func() { e.inst = e.ctor() })
	return e.inst, true
}

// Names returns the sorted list of registered ecosystem names.
func Names() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	out := make([]string, 0, len(entries))
	for k := range entries {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}
