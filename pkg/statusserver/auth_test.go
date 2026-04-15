package statusserver

import "testing"

func TestExtractRawToken(t *testing.T) {
	tests := []struct {
		name       string
		authHeader string
		expected   string
	}{
		{
			name:       "valid bearer token",
			authHeader: "Bearer abc123",
			expected:   "abc123",
		},
		{
			name:       "empty header",
			authHeader: "",
			expected:   "",
		},
		{
			name:       "missing token",
			authHeader: "Bearer",
			expected:   "",
		},
		{
			name:       "wrong auth scheme",
			authHeader: "Basic abc123",
			expected:   "",
		},
		{
			name:       "too many parts",
			authHeader: "Bearer abc def",
			expected:   "",
		},
		{
			name:       "multiple spaces",
			authHeader: "Bearer    abc123",
			expected:   "abc123",
		},
		{
			name:       "lowercase bearer",
			authHeader: "bearer abc123",
			expected:   "abc123",
		},
		{
			name:       "leading and trailing spaces",
			authHeader: "   Bearer abc123   ",
			expected:   "abc123",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRawToken(tt.authHeader)
			if got != tt.expected {
				t.Fatalf("extractRawToken(%q) = %q, want %q", tt.authHeader, got, tt.expected)
			}
		})
	}
}