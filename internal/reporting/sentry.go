package reporting

import (
	"context"
	"errors"
	"log"
	"net/http"
	"regexp"
	"time"

	"github.com/getsentry/sentry-go"
	sentryhttp "github.com/getsentry/sentry-go/http"
)

var uuidRx = regexp.MustCompile(`[0-9a-f]{8}-?([0-9a-f]{4}-?){3}[0-9a-f]{12}`)
var hostRx = regexp.MustCompile(`\[:{0,2}([0-9a-f]{0,4}:?){1,8}\]:\d+`)

func sanitizeError(err string) string {
	err = uuidRx.ReplaceAllString(err, "<uuid>")
	err = hostRx.ReplaceAllString(err, "<host>")
	return err
}

func Report(ctx context.Context, err error, extras ...map[string]string) {
	hub := sentry.GetHubFromContext(ctx)
	if hub == nil {
		log.Println("Failed to get Sentry hub from context")
		return
	}

	hub.WithScope(func(scope *sentry.Scope) {
		for _, extra := range extras {
			if extra == nil {
				continue
			}
			for key, value := range extra {
				scope.SetExtra(key, value)
			}
		}

		if err == nil {
			err = errors.New("No error provided")
		}

		scope.SetFingerprint([]string{sanitizeError(err.Error())})
		hub.CaptureException(err)
	})
}

func addTagsMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		hub := sentry.GetHubFromContext(r.Context())
		if hub == nil {
			log.Println("Failed to get Sentry hub from context")
			next(w, r)
			return
		}

		hub.ConfigureScope(func(scope *sentry.Scope) {
			scope.SetTag("user-agent", r.Header.Get("User-Agent"))

			uuid := r.URL.Query().Get("uuid")
			if uuid == "" {
				uuid = "<missing>"
			}
			scope.SetTag("uuid", uuid)

			userId := r.Header.Get("X-User-Id")
			if userId != "" {
				scope.SetUser(sentry.User{ID: userId})
			}
		})

		next(w, r)
	}
}

func InitSentryMiddleware(sentryDSN string) (func(http.HandlerFunc) http.HandlerFunc, func(), error) {
	err := sentry.Init(sentry.ClientOptions{
		Dsn:              sentryDSN,
		EnableTracing:    true,
		TracesSampleRate: 1.0 / 100.0,
	})
	if err != nil {
		return nil, nil, err
	}

	sentryHandler := sentryhttp.New(sentryhttp.Options{})

	// Wrap sentry middleware in a http.HandlerFunc
	middleware := func(next http.HandlerFunc) http.HandlerFunc {
		withAddTags := addTagsMiddleware(next)
		return func(w http.ResponseWriter, r *http.Request) {
			sentryHandler.HandleFunc(withAddTags).ServeHTTP(w, r)
		}
	}

	flush := func() {
		sentry.Flush(5 * time.Second)
	}

	return middleware, flush, nil
}
