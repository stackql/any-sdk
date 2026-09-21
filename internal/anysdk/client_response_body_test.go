package anysdk

import (
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"
)

type closeTrackingBody struct {
	io.Reader
	closed  bool
	readErr error
}

func (b *closeTrackingBody) Read(p []byte) (int, error) {
	if b.readErr != nil {
		return 0, b.readErr
	}
	return b.Reader.Read(p)
}

func (b *closeTrackingBody) Close() error {
	b.closed = true
	return nil
}

func assertBodyReadable(t *testing.T, response *http.Response, expected string) {
	t.Helper()
	b, err := io.ReadAll(response.Body)
	if err != nil {
		t.Fatalf("read replacement body: %v", err)
	}
	if string(b) != expected {
		t.Fatalf("replacement body = %q, expected %q", string(b), expected)
	}
}

// The inspected body is swapped for an in-memory copy, so a downstream Close()
// never reaches the original. Unless the original is closed at the swap,
// net/http leaves the client timeout timer running and then cancels the
// spent request (stackql/any-sdk#143).
func TestParseReponseBodyIfPresent_ClosesOriginalBody(t *testing.T) {
	original := &closeTrackingBody{Reader: strings.NewReader(`{"ok":true}`)}
	response := &http.Response{StatusCode: http.StatusOK, Body: original}
	rv, err := parseReponseBodyIfPresent(response)
	if err != nil {
		t.Fatalf("parseReponseBodyIfPresent: %v", err)
	}
	if rv != `http response status code: 200, response body: {"ok":true}` {
		t.Fatalf("unexpected rendering: %q", rv)
	}
	if !original.closed {
		t.Fatalf("expected original response body to be closed")
	}
	assertBodyReadable(t, response, `{"ok":true}`)
}

func TestParseReponseBodyIfErroneous_ClosesOriginalBody(t *testing.T) {
	original := &closeTrackingBody{Reader: strings.NewReader(`{"error":"nope"}`)}
	response := &http.Response{StatusCode: http.StatusForbidden, Body: original}
	rv, err := parseReponseBodyIfErroneous(response)
	if err != nil {
		t.Fatalf("parseReponseBodyIfErroneous: %v", err)
	}
	if rv != `http response status code: 403, response body: {"error":"nope"}` {
		t.Fatalf("unexpected rendering: %q", rv)
	}
	if !original.closed {
		t.Fatalf("expected original response body to be closed")
	}
	assertBodyReadable(t, response, `{"error":"nope"}`)
}

// A non-erroneous response is not inspected, so its body must be left alone.
func TestParseReponseBodyIfErroneous_LeavesSuccessBodyUntouched(t *testing.T) {
	original := &closeTrackingBody{Reader: strings.NewReader(`{"ok":true}`)}
	response := &http.Response{StatusCode: http.StatusOK, Body: original}
	rv, err := parseReponseBodyIfErroneous(response)
	if err != nil {
		t.Fatalf("parseReponseBodyIfErroneous: %v", err)
	}
	if rv != "" {
		t.Fatalf("expected empty rendering for success response, got %q", rv)
	}
	if original.closed {
		t.Fatalf("expected untouched success body to remain open")
	}
	if response.Body != original {
		t.Fatalf("expected success body not to be replaced")
	}
}

// On a read failure the body is left in place for the caller, as before.
func TestDrainResponseBody_ReadErrorLeavesBodyInPlace(t *testing.T) {
	readErr := errors.New("boom")
	original := &closeTrackingBody{Reader: strings.NewReader(""), readErr: readErr}
	response := &http.Response{StatusCode: http.StatusOK, Body: original}
	_, err := drainResponseBody(response)
	if !errors.Is(err, readErr) {
		t.Fatalf("expected read error, got %v", err)
	}
	if response.Body != original {
		t.Fatalf("expected body not to be replaced on read error")
	}
}
