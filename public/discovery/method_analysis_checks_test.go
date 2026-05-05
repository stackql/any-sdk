package discovery

import "testing"

func TestHasAdjacentPathParams(t *testing.T) {
	cases := []struct {
		name string
		path string
		want bool
	}{
		// Safe: literal segment separates the two placeholders.
		{"two params with literal between", "/v1/{parent}/locations/{location}", false},
		{"single param", "/v1/{name}/keys", false},
		{"no params", "/health", false},
		{"empty", "", false},
		// Violations: nothing or only `/` between two placeholders.
		{"slash-only between", "/v1/{a}/{b}", true},
		{"directly adjacent", "/v1/{a}{b}", true},
		{"multi-slash between", "/v1/{a}//{b}", true},
		// Three params: only the offending pair needs to violate.
		{"three params, last pair violates", "/v1/{a}/x/{b}/{c}", true},
		{"three params, all anchored by literals", "/v1/{a}/x/{b}/y/{c}", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := hasAdjacentPathParams(c.path); got != c.want {
				t.Fatalf("hasAdjacentPathParams(%q) = %v, want %v", c.path, got, c.want)
			}
		})
	}
}
