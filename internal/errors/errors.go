package errors

import "errors"

var (
	RetriableError         = errors.New("(retriable)")
	APIServerError         = errors.New("Server error")
	APIClientError         = errors.New("Client error")
	RatelimitExceededError = errors.New("Ratelimit exceeded")
	BadGateway             = errors.New("Bad Gateway")
	ServiceUnavailable     = errors.New("Service Unavailable")
	GatewayTimeout         = errors.New("Gateway Timeout")
)
