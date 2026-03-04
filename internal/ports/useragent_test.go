package ports_test

import (
	"net/http"
	"testing"

	"github.com/Amund211/flashlight/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestGetUserAgent(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name      string
		header    string
		hasHeader bool
		userAgent string
	}{
		{
			name:      "standard user agent",
			header:    "Prism/1.0",
			hasHeader: true,
			userAgent: "Prism/1.0",
		},
		{
			name:      "another user agent",
			header:    "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
			hasHeader: true,
			userAgent: "Mozilla/5.0 (Windows NT 10.0; Win64; x64)",
		},
		{
			name:      "empty user agent header",
			header:    "",
			hasHeader: true,
			userAgent: "<missing>",
		},
		{
			name:      "missing user agent header",
			hasHeader: false,
			userAgent: "<missing>",
		},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest("GET", "/", nil)
			require.NoError(t, err)
			if c.hasHeader {
				req.Header.Set("User-Agent", c.header)
			}
			require.Equal(t, c.userAgent, ports.GetUserAgent(req))
		})
	}
}
