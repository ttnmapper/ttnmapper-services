// Package testutil supplies isolated dependencies for tests. No request reaches a socket.
package testutil

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

type Transport func(*http.Request) (*http.Response, error)

func (f Transport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

// HTTP replaces the default transport for the duration of a serial test.
func HTTP(t *testing.T, f Transport) {
	t.Helper()
	old := http.DefaultTransport
	http.DefaultTransport = f
	t.Cleanup(func() { http.DefaultTransport = old })
}
func JSON(body string) *http.Response {
	return &http.Response{StatusCode: 200, Header: http.Header{"Content-Type": []string{"application/json"}}, Body: io.NopCloser(strings.NewReader(body))}
}
