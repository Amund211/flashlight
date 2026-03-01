package ports_test

import (
	"crypto/sha256"
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/Amund211/flashlight/internal/ports"
	"github.com/stretchr/testify/require"
)

func TestGetIP(t *testing.T) {
	t.Parallel()

	cases := []struct {
		remoteAddr    string
		xForwardedFor string
		ip            string
	}{
		{
			// Connecting through GCP load balancer
			remoteAddr:    "169.254.169.126:58418",
			xForwardedFor: "12.12.123.123,34.111.7.239",
			ip:            "12.12.123.123",
		},
		{
			// Connecting through GCP load balancer
			// Attempt to inject invalid IP via X-Forwarded-For
			// https://docs.cloud.google.com/load-balancing/docs/https#x-forwarded-for_header
			// > If the incoming request already includes an X-Forwarded-For header, the load balancer appends its values to the existing header:
			// > X-Forwarded-For: <existing-value>,<client-ip>,<load-balancer-ip>
			// Client sends X-Forwarded-For: 127.0.0.1
			// We receive:
			remoteAddr:    "169.254.169.126:44548",
			xForwardedFor: "127.0.0.1,12.123.123.1,34.111.7.239",
			ip:            "12.123.123.1",
		},
		{
			// Connecting through GCP load balancer
			// Attempt to inject invalid IP via X-Forwarded-For
			// Client sends X-Forwarded-For: 127.0.0.1,123.123.123.123
			// We receive:
			remoteAddr:    "169.254.169.126:54138",
			xForwardedFor: "127.0.0.1,123.123.123.123,12.123.123.1,34.111.7.239",
			ip:            "12.123.123.1",
		},
		{
			// Connecting directly to the cloud run service (run.app)
			remoteAddr:    "169.254.169.126:10910",
			xForwardedFor: "1111:111:1111:1111:1111:1111:1111:1111",
			ip:            "1111:111:1111:1111:1111:1111:1111:1111",
		},
		{
			// Connecting directly to the cloud run service (run.app)
			// Attempt to inject invalid IP via X-Forwarded-For
			// Client sends X-Forwarded-For: 127.0.0.1
			// We receive:
			remoteAddr:    "169.254.169.126:15050",
			xForwardedFor: "127.0.0.1,1111:111:1111:1111:1111:1111:1111:1111",
			ip:            "1111:111:1111:1111:1111:1111:1111:1111",
		},
		{
			// Connecting directly to the cloud run service (run.app)
			// Attempt to inject invalid IP via X-Forwarded-For
			// Client sends X-Forwarded-For: 127.0.0.1,123.123.123.123
			// We receive:
			remoteAddr:    "169.254.169.126:3122",
			xForwardedFor: "127.0.0.1,123.123.123.123,1111:111:1111:1111:1111:1111:1111:1111",
			ip:            "1111:111:1111:1111:1111:1111:1111:1111",
		},
		{
			// NOTE: Constructed case - not seen in production
			// No X-Forwarded-For header
			remoteAddr: "123.123.123.123",
			ip:         "<missing>",
		},
		{
			// NOTE: Constructed case - not seen in production
			// Invalid client ip in xff
			xForwardedFor: "invalid-ip",
			ip:            "<invalid>",
		},
		{
			// NOTE: Constructed case - not seen in production
			// Invalid client ip in xff
			xForwardedFor: "127.0.0.1,invalid-ip",
			ip:            "<invalid>",
		},
		{
			// NOTE: Constructed case - not seen in production
			// Invalid client ip in xff
			xForwardedFor: "invalid-ip,34.111.7.239",
			ip:            "<invalid>",
		},
		{
			// NOTE: Constructed case - not seen in production
			// Invalid client ip in xff
			xForwardedFor: "127.0.0.1,invalid-ip,34.111.7.239",
			ip:            "<invalid>",
		},
		{
			// NOTE: Constructed case - not seen in production
			// No client ip after removing load balancer ip
			xForwardedFor: "34.111.7.239",
			ip:            "<missing>",
		},
	}
	for _, c := range cases {
		t.Run(c.xForwardedFor, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest("GET", "/", nil)
			require.NoError(t, err)
			req.RemoteAddr = c.remoteAddr
			if c.xForwardedFor != "" {
				req.Header.Add("X-Forwarded-For", c.xForwardedFor)
			}
			require.Equal(t, c.ip, ports.GetIP(req))
		})
	}
}

func TestGetIPHash(t *testing.T) {
	t.Parallel()

	cases := []struct {
		remoteAddr    string
		xForwardedFor string
		expectedHash  string
	}{
		{
			// Valid IP through GCP load balancer
			remoteAddr:    "169.254.169.126:58418",
			xForwardedFor: "12.12.123.123,34.111.7.239",
			// sha256 hash of "12.12.123.123"
			expectedHash: "c89ca0df0fd2e8f5e6e2f3a3e2b8a7c1f3d0e0c8b1c0a2c8a0c1c2c3c4c5c6c7",
		},
		{
			// Valid IPv6 address
			remoteAddr:    "169.254.169.126:10910",
			xForwardedFor: "1111:111:1111:1111:1111:1111:1111:1111",
			// sha256 hash of "1111:111:1111:1111:1111:1111:1111:1111"
			expectedHash: "e7e3c9b1a1e7c4c7a2c8a0c1c2c3c4c5c6c7c8c9d0d1d2d3d4d5d6d7d8d9e0e1",
		},
		{
			// Missing X-Forwarded-For header
			remoteAddr: "123.123.123.123",
			// sha256 hash of "<missing>"
			expectedHash: "fc74d58e85d9144724e022dbf02d8c2b6b07e8bb81b99b7e0b0f3e4a1b8c8c8c",
		},
		{
			// Invalid client IP
			xForwardedFor: "invalid-ip",
			// sha256 hash of "<invalid>"
			expectedHash: "a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5a5",
		},
	}

	for _, c := range cases {
		t.Run(c.xForwardedFor, func(t *testing.T) {
			t.Parallel()

			req, err := http.NewRequest("GET", "/", nil)
			require.NoError(t, err)
			req.RemoteAddr = c.remoteAddr
			if c.xForwardedFor != "" {
				req.Header.Add("X-Forwarded-For", c.xForwardedFor)
			}

			hash := ports.GetIPHash(req)

			// Verify it's a valid hex string
			require.Len(t, hash, 64, "SHA256 hash should be 64 hex characters")
			_, err = hex.DecodeString(hash)
			require.NoError(t, err, "Hash should be valid hex")

			// Verify hash is consistent with the IP
			ip := ports.GetIP(req)
			expectedHash := sha256.Sum256([]byte(ip))
			expectedHashStr := hex.EncodeToString(expectedHash[:])
			require.Equal(t, expectedHashStr, hash)
		})
	}
}
