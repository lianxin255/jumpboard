package appt

import "testing"

// A stored key belongs to the stored endpoint; a typed address that is not
// that endpoint must not inherit the credential.
func TestSameEndpoint(t *testing.T) {
	cases := []struct {
		a, b string
		want bool
	}{
		{"https://api.example.com", "https://api.example.com/", true},
		{"https://api.example.com/v1", "https://api.example.com", true},
		{"https://api.example.com/v1/", "https://api.example.com/", true},
		{"http://127.0.0.1:3425", "http://127.0.0.1:3425/v1", true},
		{"http://127.0.0.1:3425", "http://127.0.0.1:9999", false},
		{"https://api.example.com", "http://api.example.com", false},
		{"https://api.example.com", "https://evil.example.com", false},
		{"", "", false},
		{"https://api.example.com", "", false},
	}
	for _, c := range cases {
		if got := sameEndpoint(c.a, c.b); got != c.want {
			t.Errorf("sameEndpoint(%q, %q) = %v, want %v", c.a, c.b, got, c.want)
		}
	}
}
