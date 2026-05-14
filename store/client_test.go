package store

import (
	"io"
	"net/http"
	"strings"
	"testing"
)

func TestNewClientDefaultTransport(t *testing.T) {
	client := NewClient()
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	// Default uses 30 workers, so pool should be 40.
	if transport.MaxIdleConns != 40 {
		t.Errorf("MaxIdleConns = %d, want 40", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 40 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 40", transport.MaxIdleConnsPerHost)
	}
}

func TestNewClientWithWorkersTransport(t *testing.T) {
	client := NewClientWithWorkers(50)
	transport, ok := client.httpClient.Transport.(*http.Transport)
	if !ok {
		t.Fatal("expected *http.Transport")
	}
	if transport.MaxIdleConns != 60 {
		t.Errorf("MaxIdleConns = %d, want 60", transport.MaxIdleConns)
	}
	if transport.MaxIdleConnsPerHost != 60 {
		t.Errorf("MaxIdleConnsPerHost = %d, want 60", transport.MaxIdleConnsPerHost)
	}
}

func TestNeedsRefreshWithRefreshCode(t *testing.T) {
	body := `{"error_list": [{"code": "macaroon-needs-refresh"}]}`
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	if !needsRefresh(resp) {
		t.Error("expected needsRefresh to return true for macaroon-needs-refresh")
	}
}

func TestNeedsRefreshWithHyphenatedKey(t *testing.T) {
	body := `{"error-list": [{"code": "macaroon-needs-refresh"}]}`
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	if !needsRefresh(resp) {
		t.Error("expected needsRefresh to return true for error-list variant")
	}
}

func TestNeedsRefreshOtherError(t *testing.T) {
	body := `{"error_list": [{"code": "some-other-error"}]}`
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	if needsRefresh(resp) {
		t.Error("expected needsRefresh to return false for non-refresh error")
	}
}

func TestNeedsRefreshEmptyBody(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("")),
	}

	if needsRefresh(resp) {
		t.Error("expected needsRefresh to return false for empty body")
	}
}

func TestNeedsRefreshInvalidJSON(t *testing.T) {
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader("not json")),
	}

	if needsRefresh(resp) {
		t.Error("expected needsRefresh to return false for invalid JSON")
	}
}

func TestNeedsRefreshEmptyErrorList(t *testing.T) {
	body := `{"error_list": []}`
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	if needsRefresh(resp) {
		t.Error("expected needsRefresh to return false for empty error list")
	}
}

func TestNeedsRefreshMultipleErrors(t *testing.T) {
	body := `{"error_list": [{"code": "other"}, {"code": "macaroon-needs-refresh"}]}`
	resp := &http.Response{
		Body: io.NopCloser(strings.NewReader(body)),
	}

	if !needsRefresh(resp) {
		t.Error("expected needsRefresh to return true when refresh code is among multiple errors")
	}
}
