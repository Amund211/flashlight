package app

import (
	"context"
	"fmt"
	"net/http"
	"slices"
	"strings"
	"time"

	"github.com/Amund211/flashlight/internal/domain"
	"github.com/Amund211/flashlight/internal/logging"
	"github.com/Amund211/flashlight/internal/ratelimiting"
	"github.com/Amund211/flashlight/internal/reporting"
)

type userRepository interface {
	RegisterVisit(ctx context.Context, userID string) (domain.User, error)
}

type RegisterUserVisit func(ctx context.Context, userID string) (domain.User, error)

func BuildRegisterUserVisit(repo userRepository) RegisterUserVisit {
	return func(ctx context.Context, userID string) (domain.User, error) {
		user, err := repo.RegisterVisit(ctx, userID)
		if err != nil {
			// NOTE: User repository handles its own error reporting
			return domain.User{}, fmt.Errorf("failed to register user visit in repository: %w", err)
		}
		return user, nil
	}
}

func BuildRegisterUserVisitMiddleware(registerUserVisit RegisterUserVisit) func(http.HandlerFunc) http.HandlerFunc {
	badIps := []string{
		// Put ips here
	}
	badUAs := []string{
		// Put user agents here
	}
	return func(next http.HandlerFunc) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			go func() {
				// NOTE: Since we're doing this in a goroutine, we want a context
				//       that won't get cancelled when the request ends
				ctx, cancel := context.WithTimeout(
					context.WithoutCancel(r.Context()),
					1*time.Second,
				)
				defer cancel()

				userID := r.Header.Get("X-User-Id")
				if len(userID) < 20 {
					userID = "<short user id>"
				}

				if userID == "" {
					userID = "<missing>"
				}

				userAgent := r.UserAgent()
				if userAgent == `insert-bad` ||
					userAgent == `insert-bad2` {
					userID = "<bad actor user id>"
				}

				_, _ = registerUserVisit(ctx, userID)
			}()

			{
				ctx := r.Context()

				xff := r.Header.Get("X-Forwarded-For")
				var clientIP string
				if xff != "" {
					// Split the header value by comma
					ips := strings.Split(xff, ",")
					// Trim any leading/trailing whitespace from the first IP
					clientIP = strings.TrimSpace(ips[0])
				} else {
					// Fallback to RemoteAddr if the header is not present (e.g., local development)
					logging.FromContext(ctx).WarnContext(ctx, "X-Forwarded-For header missing; using RemoteAddr")
					clientIP = r.RemoteAddr
				}

				ipKey := ratelimiting.IPKeyFunc(r)
				if len(ipKey) >= 4 {
					ipKey = ipKey[4:]
				}
				if ipKey != clientIP {
					logging.FromContext(ctx).WarnContext(
						ctx,
						"Mismatch between extracted client IP and rate limiter IP key",
						"clientIP", clientIP,
						"ipKey", ipKey,
						"xff", xff,
					)
					reporting.Report(
						ctx,
						fmt.Errorf("mismatch between extracted client IP and rate limiter IP key"),
						map[string]string{
							"clientIP":            clientIP,
							"ipKey":               ipKey,
							"headerXForwardedFor": xff,
							"remoteAddr":          r.RemoteAddr,
							"method":              r.Method,
							"userAgent":           r.UserAgent(),
							"url":                 r.URL.String(),
						},
					)
				}

				isBadIP := slices.Contains(badIps, ipKey)

				userAgent := r.UserAgent()
				isBadUA := slices.Contains(badUAs, userAgent)

				userID := r.Header.Get("X-User-Id")
				isBadUserID := userID == ""

				logging.FromContext(ctx).InfoContext(
					ctx,
					"Request info",
					"xff", xff,
					"clientIP", clientIP,
					"ipKey", ipKey,
					"remoteAddr", r.RemoteAddr,
					"userAgentRequestInfo", userAgent,
					"userID", userID,
					"isBadIP", isBadIP,
					"isBadUA", isBadUA,
					"isBadUserID", isBadUserID,
					"fullPath", r.URL.Path,
					"query", r.URL.RawQuery,
				)

				if isBadIP || isBadUA || isBadUserID {
					// All bad actors:
					// https://console.cloud.google.com/logs/query;query=jsonPayload.msg%3D%22Request%20from%20bad%20actor%22;cursorTimestamp=2026-01-18T19:24:33.323617634Z;duration=P30D?project=prism-overlay
					// New bad ips:
					// https://console.cloud.google.com/logs/query;query=jsonPayload.msg%3D%22Request%20from%20bad%20actor%22%0AjsonPayload.isBadIP%3Dfalse;cursorTimestamp=2026-01-18T19:24:33.323617634Z;duration=P30D?project=prism-overlay
					// Known bad ips:
					// https://console.cloud.google.com/logs/query;query=jsonPayload.msg%3D%22Request%20from%20bad%20actor%22%0AjsonPayload.isBadIP%3Dtrue;cursorTimestamp=2026-01-18T19:24:33.323617634Z;duration=P30D?project=prism-overlay
					logging.FromContext(ctx).WarnContext(
						ctx,
						"Request from bad actor",
						"xff", xff,
						"clientIP", clientIP,
						"remoteAddr", r.RemoteAddr,
						"userAgent", userAgent,
						"userID", userID,
						"isBadIP", isBadIP,
						"isBadUA", isBadUA,
						"isBadUserID", isBadUserID,
						"fullPath", r.URL.Path,
						"query", r.URL.RawQuery,
						"blocked", true,
					)

					http.Error(w, `{"success": false, "detail": "This API does not allow third-party use. Reach out on the Prism discord if you have questions :^) (https://discord.gg/k4FGUnEHYg)"}`, http.StatusBadRequest)
					return
				}
			}

			next(w, r)
		}
	}
}
