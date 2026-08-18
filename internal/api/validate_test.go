package api

import "testing"

func TestValidateBaseURL(t *testing.T) {
	accepted := map[string]string{
		"":                                DefaultBaseURL,
		"   ":                             DefaultBaseURL,
		"https://api.decionis.com":        "https://api.decionis.com",
		"https://api.decionis.com/":       "https://api.decionis.com",
		"https://plane.internal.corp/v1/": "https://plane.internal.corp/v1",
		"http://127.0.0.1:9999":           "http://127.0.0.1:9999",
		"http://localhost:8080/base":      "http://localhost:8080/base",
	}
	for input, want := range accepted {
		got, err := ValidateBaseURL(input)
		if err != nil {
			t.Errorf("ValidateBaseURL(%q) unexpected error: %v", input, err)
			continue
		}
		if got != want {
			t.Errorf("ValidateBaseURL(%q) = %q, want %q", input, got, want)
		}
	}

	rejected := []string{
		"http://api.decionis.com",            // plain http off loopback
		"https://user:pass@api.decionis.com", // credentials
		"https://api.decionis.com?x=1",       // query survives nothing
		"https://api.decionis.com#frag",      // fragment
		"ftp://api.decionis.com",             // scheme
		"api.decionis.com",                   // scheme-less
		"https://",                           // no host
		"http://localhost.evil.example",      // loopback look-alike
	}
	for _, input := range rejected {
		if _, err := ValidateBaseURL(input); err == nil {
			t.Errorf("ValidateBaseURL(%q) must be rejected", input)
		}
	}
}
