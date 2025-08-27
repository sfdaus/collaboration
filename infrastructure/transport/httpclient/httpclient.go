package httpclient

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type Client struct {
	baseURL string
	token   string
	http    *http.Client

	// default headers applied to every request (can be overridden per request)
	defaultHeaders http.Header

	// retry config
	maxAttempts int
}

// Option pattern
type Option func(*Client)

func WithAuthToken(token string) Option {
	return func(c *Client) { c.token = token }
}
func WithHTTPClient(h *http.Client) Option {
	return func(c *Client) { c.http = h }
}
func WithDefaultHeader(k, v string) Option {
	return func(c *Client) {
		if c.defaultHeaders == nil {
			c.defaultHeaders = http.Header{}
		}
		c.defaultHeaders.Set(k, v)
	}
}
func WithMaxAttempts(n int) Option {
	return func(c *Client) {
		if n < 1 {
			n = 1
		}
		c.maxAttempts = n
	}
}

// New returns a client with sane defaults.
// baseURL can be "" if you always pass absolute URLs in Request.Path.
func New(baseURL string, opts ...Option) *Client {
	tr := &http.Transport{
		MaxIdleConns:          256,
		MaxIdleConnsPerHost:   64,
		IdleConnTimeout:       90 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ForceAttemptHTTP2:     true,
		DialContext: (&net.Dialer{
			Timeout:   5 * time.Second,
			KeepAlive: 60 * time.Second,
		}).DialContext,
		// tweak if you use internal self-signed certs:
		TLSClientConfig: &tls.Config{InsecureSkipVerify: false},
	}
	c := &Client{
		baseURL: strings.TrimRight(baseURL, "/"),
		http: &http.Client{
			Timeout:   8 * time.Second, // hard per-request timeout
			Transport: tr,
		},
		maxAttempts: 3,
	}
	// sensible defaults
	c.defaultHeaders = http.Header{}
	c.defaultHeaders.Set("Accept", "application/json")

	for _, o := range opts {
		o(c)
	}
	return c
}

// Request represents an outgoing HTTP call.
type Request struct {
	Method string      // GET, POST, PUT, PATCH, DELETE
	Path   string      // absolute ("http://...") or relative (appended to baseURL)
	Query  url.Values  // optional query params
	Header http.Header // optional headers
	// Body precedence: Reader > Raw > JSON
	BodyJSON   any       // marshaled to JSON if set and BodyRaw/BodyReader nil
	BodyRaw    []byte    // sent as-is if BodyReader nil
	BodyReader io.Reader // streamed if set (caller sets Content-Type)
	// Idempotency key (optional). If empty on mutating methods, it will be auto-generated.
	IdempotencyKey string
}

// Response wraps an HTTP response with helpers.
type Response struct {
	StatusCode int
	Header     http.Header
	Body       []byte
}

// DecodeJSON decodes response body into v (struct/map).
func (r *Response) DecodeJSON(v any) error {
	if len(r.Body) == 0 {
		return io.EOF
	}
	return json.Unmarshal(r.Body, v)
}

// JSON is a generic helper: var out T; out, err := resp.JSON[T]()
func JSONAs[T any](r *Response) (T, error) {
	var zero T
	if r == nil || len(r.Body) == 0 {
		return zero, io.EOF
	}
	var out T
	return out, json.Unmarshal(r.Body, &out)
}

// HTTPError represents non-2xx responses.
type HTTPError struct {
	Status int
	Body   string
}

func (e *HTTPError) Error() string {
	preview := e.Body
	if len(preview) > 400 {
		preview = preview[:400] + "…"
	}
	return fmt.Sprintf("http error: status=%d body=%s", e.Status, preview)
}

// Do executes the request with retry/backoff for 5xx and 429.
// 2xx returns (*Response, nil). 4xx (except 429) returns (*Response, *HTTPError).
func (c *Client) Do(ctx context.Context, req Request) (*Response, error) {
	if req.Method == "" {
		req.Method = http.MethodGet
	}
	// build URL
	u := req.Path
	if !strings.HasPrefix(u, "http://") && !strings.HasPrefix(u, "https://") {
		if c.baseURL == "" {
			return nil, errors.New("no baseURL provided and request.Path is relative")
		}
		u = c.baseURL + "/" + strings.TrimLeft(u, "/")
	}
	if len(req.Query) > 0 {
		qs := req.Query.Encode()
		if strings.Contains(u, "?") {
			u += "&" + qs
		} else {
			u += "?" + qs
		}
	}

	// prepare body & headers
	var body io.Reader
	ct := "" // content-type to set if needed

	switch {
	case req.BodyReader != nil:
		body = req.BodyReader
	case len(req.BodyRaw) > 0:
		body = bytes.NewReader(req.BodyRaw)
	default:
		if req.BodyJSON != nil {
			b, err := json.Marshal(req.BodyJSON)
			if err != nil {
				return nil, err
			}
			body = bytes.NewReader(b)
			ct = "application/json"
		}
	}
	// clone headers
	h := http.Header{}
	// defaults
	for k, vs := range c.defaultHeaders {
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	// per-request
	for k, vs := range req.Header {
		for _, v := range vs {
			h.Add(k, v)
		}
	}
	// set content-type if sending JSON and not already set
	if ct != "" && h.Get("Content-Type") == "" {
		h.Set("Content-Type", ct)
	}
	// auth
	if c.token != "" && h.Get("Authorization") == "" {
		h.Set("Authorization", "Bearer "+c.token)
	}
	// tracing
	xrid := RequestIDFromContext(ctx)
	if xrid == "" {
		xrid = uuid.New().String()
	}
	h.Set("X-Request-ID", xrid)
	// idempotency
	if isMutating(req.Method) && h.Get("Idempotency-Key") == "" {
		key := req.IdempotencyKey
		if key == "" {
			key = uuid.New().String()
		}
		h.Set("Idempotency-Key", key)
	}

	var lastErr error
	attempts := c.maxAttempts
	if attempts < 1 {
		attempts = 1
	}

	for attempt := 1; attempt <= attempts; attempt++ {
		httpReq, err := http.NewRequestWithContext(ctx, req.Method, u, body)
		if err != nil {
			return nil, err
		}
		httpReq.Header = h.Clone()

		// rewind body for retries if needed
		if attempt > 1 && body != nil {
			if seeker, ok := body.(io.Seeker); ok {
				_, _ = seeker.Seek(0, io.SeekStart)
			} else {
				// cannot retry non-seekable body
				return nil, fmt.Errorf("body is not seekable; cannot retry")
			}
		}

		resp, err := c.http.Do(httpReq)
		if err != nil {
			if attempt < attempts && isRetryableNetErr(err) {
				sleepBackoff(attempt, 0)
				lastErr = err
				continue
			}
			return nil, err
		}

		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1<<20)) // 1MB cap
		_ = resp.Body.Close()

		rr := &Response{StatusCode: resp.StatusCode, Header: resp.Header, Body: b}

		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return rr, nil
		}

		if resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode <= 599) {
			if attempt < attempts {
				sleepBackoff(attempt, retryAfter(resp.Header.Get("Retry-After")))
				lastErr = &HTTPError{Status: resp.StatusCode, Body: string(b)}
				continue
			}
			return rr, &HTTPError{Status: resp.StatusCode, Body: string(b)}
		}

		// 4xx (permanent)
		return rr, &HTTPError{Status: resp.StatusCode, Body: string(b)}
	}
	return nil, lastErr
}

func isMutating(m string) bool {
	switch strings.ToUpper(m) {
	case http.MethodPost, http.MethodPut, http.MethodPatch, http.MethodDelete:
		return true
	default:
		return false
	}
}

func isRetryableNetErr(err error) bool {
	var ne net.Error
	if errors.As(err, &ne) && (ne.Timeout() || ne.Temporary()) {
		return true
	}
	msg := strings.ToLower(err.Error())
	return strings.Contains(msg, "connection reset") ||
		strings.Contains(msg, "broken pipe") ||
		strings.Contains(msg, "unexpected eof") ||
		strings.Contains(msg, "eof")
}

func retryAfter(h string) time.Duration {
	// supports numeric seconds; ignore HTTP-date for simplicity
	if h == "" {
		return 0
	}
	// parse int
	var secs int
	for _, ch := range h {
		if ch < '0' || ch > '9' {
			return 0
		}
	}
	fmt.Sscanf(h, "%d", &secs)
	if secs <= 0 {
		return 0
	}
	return time.Duration(secs) * time.Second
}

func sleepBackoff(attempt int, extra time.Duration) {
	// simple exponential backoff with bounded jitter
	base := time.Duration(attempt*200) * time.Millisecond // 200ms, 400ms, 600ms...
	wait := base
	if extra > wait {
		wait = extra
	}
	time.Sleep(wait)
}

// ---- Tracing helpers (ctx) ----

type ctxKey string

const requestIDKey ctxKey = "x-request-id"

func WithRequestID(ctx context.Context, id string) context.Context {
	return context.WithValue(ctx, requestIDKey, id)
}
func RequestIDFromContext(ctx context.Context) string {
	if v := ctx.Value(requestIDKey); v != nil {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}
