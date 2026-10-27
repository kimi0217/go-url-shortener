package shortener

import (
	"strings"
	"testing"
)

func TestGenerate_LengthAndAlphabet(t *testing.T) {
	// Randomness means one sample could pass by luck, so check many.
	for range 1000 {
		code, err := Generate()
		if err != nil {
			t.Fatalf("Generate: unexpected error: %v", err)
		}
		if len(code) != Length {
			t.Fatalf("len(%q) = %d, want %d", code, len(code), Length)
		}
		for _, c := range code {
			if !strings.ContainsRune(alphabet, c) {
				t.Fatalf("code %q contains %q, which is not in the alphabet", code, c)
			}
		}
	}
}

func TestGenerate_Unique(t *testing.T) {
	// With 62^6 possible codes, the chance of any repeat among 1000 is
	// about 1 in 100,000, so a duplicate here almost certainly means the
	// generator is broken (e.g. returning a constant).
	const n = 1000
	seen := make(map[string]bool, n)
	for range n {
		code, err := Generate()
		if err != nil {
			t.Fatalf("Generate: unexpected error: %v", err)
		}
		if seen[code] {
			t.Fatalf("duplicate code %q after %d generations", code, len(seen))
		}
		seen[code] = true
	}
}
