package client

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"testing"
	"time"
)

func TestURLValidation(t *testing.T) {
	for _, origin := range []string{"http://example.com", "https://user:secret@example.com", "https://example.com/other", "https://example.com?token=x", "https://example.com/#x", "//example.com"} {
		if _, err := New(origin, "secret", "test"); err == nil {
			t.Errorf("accepted %q", origin)
		}
	}
	for _, origin := range []string{"https://example.com", "https://example.com/v1/", "http://localhost:3001", "http://[::1]:3001"} {
		if _, err := New(origin, "secret", "test"); err != nil {
			t.Errorf("rejected %q: %v", origin, err)
		}
	}
}
func TestRetryPolicy(t *testing.T) {
	for _, tc := range []struct {
		name, method, path string
		status, want       int
	}{{"idempotent create", "POST", "/v1/xcloud/instances", 503, 2}, {"unsafe create", "POST", "/v1/xcloud/volumes", 503, 1}, {"read", "GET", "/v1/regions", 503, 2}, {"rate limited create", "POST", "/v1/xcloud/volumes", 429, 2}, {"permission", "GET", "/v1/regions", 403, 1}} {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			key := ""
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if r.Header.Get("Authorization") != "Bearer test-secret" {
					t.Error("missing bearer token")
				}
				if calls == 1 {
					key = r.Header.Get("Idempotency-Key")
					w.WriteHeader(tc.status)
					fmt.Fprint(w, `{"detail":"try later"}`)
					return
				}
				if r.Header.Get("Idempotency-Key") != key {
					t.Error("retry changed idempotency key")
				}
				fmt.Fprint(w, `{"id":"ok"}`)
			}))
			defer server.Close()
			c, _ := New(server.URL, "test-secret", "test")
			_, err := c.Send(context.Background(), tc.method, tc.path, nil, Object{"name": "test"})
			if calls != tc.want {
				t.Errorf("calls=%d, want %d", calls, tc.want)
			}
			if tc.want == 2 && err != nil {
				t.Fatal(err)
			}
			if tc.want == 1 && err == nil {
				t.Fatal("expected failure")
			}
			if tc.path == "/v1/xcloud/instances" && key == "" {
				t.Fatal("missing idempotency key")
			}
		})
	}
}
func TestRedirectAndRedaction(t *testing.T) {
	leaked := false
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked = true }))
	defer target.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/v1/error" {
			w.Header().Set("X-Request-ID", "trace-123")
			w.WriteHeader(403)
			fmt.Fprint(w, `{"detail":"rejected test-secret"}`)
			return
		}
		http.Redirect(w, r, target.URL, 307)
	}))
	defer server.Close()
	c, _ := New(server.URL, "test-secret", "test")
	if _, err := c.Get(context.Background(), "/v1/redirect", nil); err == nil {
		t.Fatal("redirect accepted")
	}
	if leaked {
		t.Fatal("followed redirect with credential")
	}
	_, err := c.Get(context.Background(), "/v1/error", nil)
	if err == nil || strings.Contains(err.Error(), "test-secret") || !strings.Contains(err.Error(), "trace-123") {
		t.Fatalf("unexpected diagnostic: %v", err)
	}
}
func TestWait(t *testing.T) {
	c := &Client{PollInterval: time.Millisecond, token: "secret"}
	calls := 0
	_, err := c.Wait(context.Background(), func(context.Context) (Object, error) {
		calls++
		if calls == 2 {
			return Object{"status": "running", "pendingAction": nil}, nil
		}
		return Object{"status": "running", "pendingAction": "create"}, nil
	}, func(o Object) bool { return o["pendingAction"] == nil }, false)
	if err != nil || calls != 2 {
		t.Fatalf("wait: %d %v", calls, err)
	}
	_, err = c.Wait(context.Background(), func(context.Context) (Object, error) { return nil, &APIError{Status: 404} }, nil, true)
	if err != nil {
		t.Fatal(err)
	}
	_, err = c.Wait(context.Background(), func(context.Context) (Object, error) {
		return Object{"status": "error", "lastError": "secret failed"}, nil
	}, func(Object) bool { return false }, false)
	if err == nil || strings.Contains(err.Error(), "secret") {
		t.Fatalf("failure not redacted: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err = c.Wait(ctx, func(context.Context) (Object, error) { return Object{"status": "pending"}, nil }, func(Object) bool { return false }, false)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected cancellation, got %v", err)
	}
}

func TestMalformedListDoesNotMeanDeleted(t *testing.T) {
	for _, body := range []string{`{}`, `{"data":null}`, `{"data":{}}`} {
		t.Run(body, func(t *testing.T) {
			s := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, body) }))
			defer s.Close()
			c, _ := New(s.URL, "test-key", "test")
			_, err := c.Find(context.Background(), "/v1/ssh-keys", nil, "id", "missing")
			if err == nil || IsNotFound(err) {
				t.Fatalf("malformed list inferred deletion: %v", err)
			}
		})
	}
}

func TestRequestSecretRedaction(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(400)
		_, _ = fmt.Fprint(w, `{"detail":"invalid credential: secret-value"}`)
	}))
	defer server.Close()
	c, _ := New(server.URL, "token", "test")
	for _, payload := range []Object{{"password": "secret-value"}, {"adminPassword": "secret-value"}, {"auth": Object{"kind": "adhoc", "password": "secret-value"}}} {
		err := c.Do(context.Background(), "POST", "/v1/xcloud/images", nil, payload, nil)
		if err == nil || strings.Contains(err.Error(), "secret-value") || !strings.Contains(err.Error(), "[REDACTED]") {
			t.Fatalf("expected redacted API error, got %v", err)
		}
	}
}

// Simulate an accepted write whose response is lost. A second call would be a
// duplicate side effect (or a 409), even when the route mounts middleware.
func TestUnprotectedWritesAreNotReplayed(t *testing.T) {
	writes := []struct{ method, path string }{
		{"POST", "/v1/xcloud/networks"}, {"DELETE", "/v1/xcloud/networks/private"},
		{"POST", "/v1/xcloud-security-groups"}, {"PATCH", "/v1/xcloud-security-groups/ssh"}, {"DELETE", "/v1/xcloud-security-groups/ssh"},
		{"POST", "/v1/xcloud/instances/id/start"}, {"POST", "/v1/xcloud/instances/id/stop"}, {"POST", "/v1/xcloud/instances/id/shutdown"},
		{"POST", "/v1/xcloud/instances/id/suspend"}, {"POST", "/v1/xcloud/instances/id/boot-mode"}, {"POST", "/v1/xcloud/instances/id/resize"},
		{"PATCH", "/v1/xcloud/instances/id"}, {"PUT", "/v1/xcloud/instances/id/tags"}, {"PUT", "/v1/xcloud/instances/id/password"},
		{"DELETE", "/v1/xcloud/instances/id"}, {"POST", "/v1/xcloud/instances/id/volumes"},
		{"POST", "/v1/xcloud/instances/"}, {"PATCH", "/v1/xcloud/instances"},
	}
	for _, write := range writes {
		for _, failure := range []string{"transport", "408", "502", "503"} {
			t.Run(write.method+" "+write.path+" "+failure, func(t *testing.T) {
				calls := 0
				c, _ := New("https://api.example", "test-key", "test")
				c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
					calls++
					if req.Header.Get("Idempotency-Key") != "" {
						t.Error("unprotected write has an idempotency key")
					}
					if failure == "transport" {
						return nil, errors.New("connection lost after accepting write")
					}
					code, _ := strconv.Atoi(failure)
					if calls > 1 {
						code = 409
					}
					return &http.Response{StatusCode: code, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"detail":"response lost after accepting write"}`))}, nil
				})
				err := c.Do(context.Background(), write.method, write.path, nil, Object{"name": "test"}, nil)
				if err == nil || calls != 1 {
					t.Fatalf("ambiguous write replayed: calls=%d, err=%v", calls, err)
				}
			})
		}
	}
}

type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestInstanceCreateTransportRetryUsesSameKey(t *testing.T) {
	c, _ := New("https://api.example", "test-key", "test")
	calls := 0
	key := ""
	c.http.Transport = roundTripFunc(func(req *http.Request) (*http.Response, error) {
		calls++
		if calls == 1 {
			key = req.Header.Get("Idempotency-Key")
			return nil, errors.New("response lost")
		}
		if key == "" || req.Header.Get("Idempotency-Key") != key {
			t.Error("missing or changed replay key")
		}
		return &http.Response{StatusCode: 201, Header: http.Header{}, Body: io.NopCloser(strings.NewReader(`{"id":"original"}`))}, nil
	})
	result, err := c.Send(context.Background(), "POST", "/v1/xcloud/instances", nil, Object{"name": "test"})
	if err != nil || calls != 2 || result["id"] != "original" {
		t.Fatalf("create replay failed: calls=%d result=%v err=%v", calls, result, err)
	}
}
