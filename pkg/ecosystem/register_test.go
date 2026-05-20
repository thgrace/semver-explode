package ecosystem

import (
	"errors"
	"testing"
)

// stubEco is a minimal Ecosystem used only in tests.
type stubEco struct{ id string }

func (s *stubEco) Name() string                         { return s.id }
func (s *stubEco) ParseVersion(string) (Version, error) { return nil, errors.New("stub") }
func (s *stubEco) ParseRange(string) (Range, error)     { return nil, errors.New("stub") }
func (s *stubEco) Registry() Registry                   { return nil }

func TestLookup_UnknownName(t *testing.T) {
	eco, ok := Lookup("test-eco-nonexistent-xyz")
	if ok {
		t.Errorf("expected ok=false for unknown name, got true (eco=%v)", eco)
	}
	if eco != nil {
		t.Errorf("expected nil Ecosystem for unknown name, got %v", eco)
	}
}

func TestLookup_ReturnsSameInstance(t *testing.T) {
	const name = "test-eco-conformance"
	Register(name, func() Ecosystem { return &stubEco{id: name} })
	t.Cleanup(func() {
		registryMu.Lock()
		delete(entries, name)
		registryMu.Unlock()
	})

	first, ok1 := Lookup(name)
	if !ok1 {
		t.Fatalf("first Lookup(%q) returned ok=false", name)
	}
	second, ok2 := Lookup(name)
	if !ok2 {
		t.Fatalf("second Lookup(%q) returned ok=false", name)
	}
	if first != second {
		t.Errorf("Lookup returned different instances: %p vs %p — caching is broken", first, second)
	}
}

func TestRegister_PanicsOnDuplicate(t *testing.T) {
	const name = "test-eco-dup"
	Register(name, func() Ecosystem { return &stubEco{id: name} })
	t.Cleanup(func() {
		registryMu.Lock()
		delete(entries, name)
		registryMu.Unlock()
	})

	defer func() {
		if r := recover(); r == nil {
			t.Errorf("expected panic on duplicate Register(%q), but did not panic", name)
		}
	}()
	Register(name, func() Ecosystem { return &stubEco{id: name} })
}
