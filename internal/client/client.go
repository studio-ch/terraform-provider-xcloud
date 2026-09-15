// Package client implements the public Cloud Console REST API only.
package client

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

const DefaultURL = "https://api.cloud.flow.swiss"

type Object = map[string]any

type Client struct {
	origin, token, userAgent       string
	http                           *http.Client
	PollInterval, OperationTimeout time.Duration
}

func New(origin, token, version string) (*Client, error) {
	u, err := url.Parse(strings.TrimRight(origin, "/"))
	if err != nil || u.Host == "" || u.User != nil || u.RawQuery != "" || u.Fragment != "" || (u.Path != "" && u.Path != "/v1") {
		return nil, errors.New("api_url must be an absolute API origin, optionally ending in /v1, without credentials, query or fragment")
	}
	if u.Scheme != "https" && !(u.Scheme == "http" && (u.Hostname() == "localhost" || u.Hostname() == "127.0.0.1" || u.Hostname() == "::1")) {
		return nil, errors.New("api_url requires HTTPS (HTTP is allowed only on loopback for development)")
	}
	u.Path = ""
	if strings.TrimSpace(token) == "" {
		return nil, errors.New("set api_token or XCLOUD_API_TOKEN (CLOUDCONSOLE_API_TOKEN is also supported)")
	}
	return &Client{origin: u.String(), token: token, userAgent: "terraform-provider-xcloud/" + version,
		http:         &http.Client{Timeout: 60 * time.Second, CheckRedirect: func(_ *http.Request, _ []*http.Request) error { return http.ErrUseLastResponse }},
		PollInterval: 5 * time.Second, OperationTimeout: 30 * time.Minute}, nil
}

type APIError struct {
	Status            int
	Detail, RequestID string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("API HTTP %d: %s (request-id: %s)", e.Status, e.Detail, e.RequestID)
}
func IsNotFound(err error) bool          { var e *APIError; return errors.As(err, &e) && e.Status == 404 }
func (c *Client) Redact(s string) string { return strings.ReplaceAll(s, c.token, "[REDACTED]") }
func (c *Client) Operation(ctx context.Context) (context.Context, context.CancelFunc) {
	return context.WithTimeout(ctx, c.OperationTimeout)
}

// Only instance creation persists a replay response. Mounting the middleware
// on a route group is insufficient: each handler must call persistIdempotency.
func supportsIdempotency(method, path string) bool {
	return method == http.MethodPost && path == "/v1/xcloud/instances"
}
func requestID() string { var b [16]byte; _, _ = rand.Read(b[:]); return hex.EncodeToString(b[:]) }
func (c *Client) Do(ctx context.Context, method, path string, query url.Values, body any, out any) error {
	ctx, cancel := c.Operation(ctx)
	defer cancel()
	if !strings.HasPrefix(path, "/v1/") {
		return errors.New("request path must use the public /v1 API")
	}
	var data []byte
	var err error
	if body != nil {
		data, err = json.Marshal(body)
		if err != nil {
			return err
		}
	}
	target := c.origin + path
	if len(query) > 0 {
		target += "?" + query.Encode()
	}
	key := ""
	if supportsIdempotency(method, path) {
		key = requestID()
	}
	safe := method == "GET" || key != ""
	for attempt := 0; attempt < 5; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, target, bytes.NewReader(data))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.token)
		req.Header.Set("User-Agent", c.userAgent)
		req.Header.Set("Accept", "application/json")
		req.Header.Set("X-Request-ID", requestID())
		if body != nil {
			req.Header.Set("Content-Type", "application/json")
		}
		if key != "" {
			req.Header.Set("Idempotency-Key", key)
		}
		res, err := c.http.Do(req)
		delay := time.Duration(1<<attempt) * 250 * time.Millisecond
		if err != nil {
			if safe && attempt < 4 && ctx.Err() == nil {
				if err = Sleep(ctx, delay); err != nil {
					return err
				}
				continue
			}
			return fmt.Errorf("public API request failed: %s", c.Redact(err.Error()))
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 16<<20))
		res.Body.Close()
		if readErr != nil {
			return fmt.Errorf("read API response (request-id: %s): %w", res.Header.Get("X-Request-ID"), readErr)
		}
		if res.StatusCode >= 200 && res.StatusCode < 300 {
			if out != nil {
				if err = json.Unmarshal(raw, out); err != nil {
					return fmt.Errorf("invalid JSON from public API (request-id: %s): %w", res.Header.Get("X-Request-ID"), err)
				}
			}
			return nil
		}
		// 429 is emitted before the route handler; even unprotected writes can retry it.
		retry := res.StatusCode == 429 || safe && (res.StatusCode == 408 || res.StatusCode >= 500)
		if retry && attempt < 4 {
			if v, e := strconv.Atoi(res.Header.Get("Retry-After")); e == nil && v > 0 {
				delay = max(delay, time.Duration(v)*time.Second)
			} else if t, e := http.ParseTime(res.Header.Get("Retry-After")); e == nil {
				delay = max(delay, time.Until(t))
			}
			if err = Sleep(ctx, delay); err != nil {
				return err
			}
			continue
		}
		var problem struct {
			Detail string `json:"detail"`
			Title  string `json:"title"`
		}
		_ = json.Unmarshal(raw, &problem)
		detail := problem.Detail
		if detail == "" {
			detail = problem.Title
		}
		if detail == "" {
			detail = http.StatusText(res.StatusCode)
		}
		return &APIError{Status: res.StatusCode, Detail: c.Redact(redactRequestSecrets(detail, data)), RequestID: res.Header.Get("X-Request-ID")}
	}
	return errors.New("API retry budget exhausted")
}
func (c *Client) Get(ctx context.Context, path string, query url.Values) (Object, error) {
	var o Object
	err := c.Do(ctx, "GET", path, query, nil, &o)
	return o, err
}
func (c *Client) Send(ctx context.Context, method, path string, query url.Values, body any) (Object, error) {
	var o Object
	err := c.Do(ctx, method, path, query, body, &o)
	return o, err
}
func (c *Client) List(ctx context.Context, path string, query url.Values) ([]Object, error) {
	var result struct {
		Data json.RawMessage `json:"data"`
	}
	if err := c.Do(ctx, "GET", path, query, nil, &result); err != nil {
		return nil, err
	}
	if len(result.Data) == 0 || string(result.Data) == "null" {
		return nil, errors.New("invalid API list: missing data array; refusing to infer deleted resources")
	}
	var rows []Object
	if err := json.Unmarshal(result.Data, &rows); err != nil {
		return nil, fmt.Errorf("invalid API list data: %w", err)
	}
	return rows, nil
}
func (c *Client) Find(ctx context.Context, path string, query url.Values, key, value string) (Object, error) {
	rows, err := c.List(ctx, path, query)
	if err != nil {
		return nil, err
	}
	for _, row := range rows {
		if row[key] == value {
			return row, nil
		}
	}
	return nil, &APIError{Status: 404, Detail: "resource not found in public API list"}
}
func Sleep(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}
func (c *Client) Wait(ctx context.Context, fetch func(context.Context) (Object, error), done func(Object) bool, deleted bool) (Object, error) {
	for {
		o, err := fetch(ctx)
		if deleted && IsNotFound(err) {
			return nil, nil
		}
		if err != nil {
			return nil, err
		}
		if o["status"] == "error" || o["state"] == "error" {
			return o, fmt.Errorf("operation failed: %s", c.Redact(fmt.Sprint(o["lastError"])))
		}
		if !deleted && done(o) {
			return o, nil
		}
		if err = Sleep(ctx, c.PollInterval); err != nil {
			return o, fmt.Errorf("waiting for asynchronous operation: %w; the operation may still complete", err)
		}
	}
}

// Servers can echo invalid field values in error details. Scrub credentials in
// this request, including nested image auth, before they reach diagnostics.
func redactRequestSecrets(detail string, data []byte) string {
	var body any
	if json.Unmarshal(data, &body) != nil {
		return detail
	}
	var walk func(any)
	walk = func(v any) {
		switch x := v.(type) {
		case map[string]any:
			for k, v := range x {
				if strings.Contains(strings.ToLower(k), "password") || strings.Contains(strings.ToLower(k), "token") || strings.EqualFold(k, "userData") {
					if secret, ok := v.(string); ok && secret != "" {
						detail = strings.ReplaceAll(detail, secret, "[REDACTED]")
						quoted, _ := json.Marshal(secret)
						detail = strings.ReplaceAll(detail, string(quoted[1:len(quoted)-1]), "[REDACTED]")
					}
				} else {
					walk(v)
				}
			}
		case []any:
			for _, v := range x {
				walk(v)
			}
		}
	}
	walk(body)
	return detail
}
