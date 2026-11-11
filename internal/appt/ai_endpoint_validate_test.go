package appt

import "testing"

// The endpoint carries the API key in an Authorization header, so only an
// http(s) address with a host may be stored.
func TestSetAIRejectsNonHTTPEndpoint(t *testing.T) {
	a := newTestApp(t)
	bad := []string{
		"api.example.com",       // no scheme
		"ftp://api.example.com", // wrong scheme
		"file:///etc/passwd",
		"http://", // no host
	}
	for _, raw := range bad {
		if err := a.SetAI(raw, "gpt-4o"); err == nil {
			t.Errorf("SetAI(%q) accepted, want a refusal", raw)
		}
	}
	good := []string{
		"http://127.0.0.1:3425",
		"https://api.example.com",
		"https://api.example.com/v1",
	}
	for _, raw := range good {
		if err := a.SetAI(raw, "gpt-4o"); err != nil {
			t.Errorf("SetAI(%q) refused: %v", raw, err)
		}
	}
	// An empty address clears the setting, which is how the panel is hidden.
	if err := a.SetAI("", ""); err != nil {
		t.Errorf("SetAI(\"\", \"\") refused: %v", err)
	}
}
