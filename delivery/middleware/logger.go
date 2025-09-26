package middleware

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime/debug"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/labstack/echo/v4"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

// RequestIDHeader is the name of the HTTP Header which contains the request id.
// Exported so that it can be changed by developers
var RequestIDHeader = "X-Request-Id"

type logFields struct {
	RemoteIP   string
	Host       string
	Method     string
	Path       string
	Body       string
	StatusCode int
	Latency    float64
	Error      error
	Stack      []byte
}

func (l *logFields) MarshalZerologObject(e *zerolog.Event) {
	e.
		Str("remote_ip", l.RemoteIP).
		Str("host", l.Host).
		Str("method", l.Method).
		Str("path", l.Path).
		Str("body", l.Body).
		Int("status_code", l.StatusCode).
		Float64("latency", l.Latency).
		Str("tag", "request")

	if l.Error != nil {
		e.Err(l.Error)
	}

	if l.Stack != nil {
		e.Bytes("stack", l.Stack)
	}
}

// Logger contains functionality of request_id, logger and recover for request traceability
func (m *Middleware) Logger(filter func(c echo.Context) bool) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {

			if filter != nil && filter(c) {
				return next(c)
			}

			// Start timer
			start := time.Now()

			// Generate request ID
			// will search for a request ID header and set into the log context
			if c.Request().Header.Get(RequestIDHeader) == "" {
				c.Request().Header.Set(RequestIDHeader, uuid.New().String())
			}

			ctx := log.With().
				Str("request_id", c.Request().Header.Get(RequestIDHeader)).
				Logger().
				WithContext(c.Request().Context())

			// Read request body
			var buf []byte
			if c.Request().Body != nil {
				buf, _ = io.ReadAll(c.Request().Body)

				// Restore the io.ReadCloser to its original state
				c.Request().Body = io.NopCloser(bytes.NewBuffer(buf))
			}

			// Create log fields
			fields := &logFields{
				RemoteIP: c.RealIP(),
				Method:   c.Request().Method,
				Host:     c.Request().Host,
				Path:     c.Request().RequestURI,
				// Body:     formatReqBody(buf),
			}

			defer func() {
				rvr := recover()

				if rvr != nil {
					if rvr == http.ErrAbortHandler {
						// We don't recover http.ErrAbortHandler so the response
						// to the client is aborted, this should not be logged
						panic(rvr)
					}

					err, ok := rvr.(error)
					if !ok {
						err = fmt.Errorf("%v", rvr)
					}

					fields.Error = err
					fields.Stack = debug.Stack()

					c.Error(err)
				}

				fields.StatusCode = c.Response().Status
				fields.Latency = float64(time.Since(start).Nanoseconds()/1e4) / 100.0

				switch {
				case rvr != nil:
					log.Ctx(ctx).Error().EmbedObject(fields).Msg("panic recover")
				case fields.StatusCode >= 500:
					log.Ctx(ctx).Error().EmbedObject(fields).Msg("server error")
				case fields.StatusCode >= 400:
					log.Ctx(ctx).Error().EmbedObject(fields).Msg("client error")
				case fields.StatusCode >= 300:
					log.Ctx(ctx).Warn().EmbedObject(fields).Msg("redirect")
				case fields.StatusCode >= 200:
					log.Ctx(ctx).Info().EmbedObject(fields).Msg("success")
				case fields.StatusCode >= 100:
					log.Ctx(ctx).Info().EmbedObject(fields).Msg("informative")
				default:
					log.Ctx(ctx).Warn().EmbedObject(fields).Msg("unknown status")
				}

			}()

			newReq := c.Request().WithContext(ctx)
			c.SetRequest(newReq)

			return next(c)
		}
	}
}

func formatReqBody(data []byte) string {
	var js map[string]interface{}
	if json.Unmarshal(data, &js) != nil {
		return string(data)
	}

	result := new(bytes.Buffer)
	if err := json.Compact(result, data); err != nil {
		log.Error().Err(err).Msg("error compacting body request json")
		return ""
	}

	return result.String()
}

const (
	ColorReset  = "\033[0m"
	ColorRed    = "\033[31m"
	ColorGreen  = "\033[32m"
	ColorYellow = "\033[33m"
	ColorBlue   = "\033[34m"
)

type bodyDumpResponseWriter struct {
	io.Writer
	http.ResponseWriter
}

func (w *bodyDumpResponseWriter) Write(b []byte) (int, error) {
	w.Writer.Write(b) // store copy
	return w.ResponseWriter.Write(b)
}

func (m *Middleware) CustomLogger() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			// ambil path yang bener (kadang c.Path() kosong sebelum match)
			path := c.Path()
			if path == "" {
				path = c.Request().URL.Path
			}
			ua := c.Request().Header.Get("User-Agent")

			// skip root, health endpoints, swagger, favicon, dan kube-probe/kubelet
			if path == "/" ||
				path == "/health" || path == "/healthz" || path == "/readyz" ||
				strings.HasPrefix(path, "/api/v1/auth/swagger") ||
				path == "/favicon.ico" ||
				strings.HasPrefix(ua, "kube-probe") || strings.HasPrefix(ua, "kubelet") {
				return next(c)
			}
			// --- END SKIP SECTION ---

			start := time.Now()

			// Generate request_id if not provided
			requestID := c.Request().Header.Get("X-Request-ID")
			if requestID == "" {
				requestID = uuid.NewString()
			}

			// Detect content type
			contentType := c.Request().Header.Get("Content-Type")
			var bodyStr string
			var bodyBytes []byte

			if strings.HasPrefix(contentType, "multipart/form-data") {
				if err := c.Request().ParseMultipartForm(10 << 20); err == nil && c.Request().MultipartForm != nil {
					files := []string{}
					for key, fhs := range c.Request().MultipartForm.File {
						for _, fh := range fhs {
							files = append(files, fmt.Sprintf("%s(name=%s, size=%d)", key, fh.Filename, fh.Size))
						}
					}
					bodyStr = fmt.Sprintf("[multipart form: %s]", strings.Join(files, ", "))
				} else {
					bodyStr = "[multipart/form-data unreadable]"
				}
				c.Request().Body = http.MaxBytesReader(c.Response(), c.Request().Body, 10<<20)
			} else {
				bodyBytes, _ = io.ReadAll(c.Request().Body)
				var compacted bytes.Buffer
				if json.Valid(bodyBytes) {
					_ = json.Compact(&compacted, bodyBytes)
					bodyBytes = compacted.Bytes()
				}
				c.Request().Body = io.NopCloser(bytes.NewBuffer(bodyBytes))
				bodyStr = string(bodyBytes)
			}

			// Capture response body
			resBody := new(bytes.Buffer)
			writer := &bodyDumpResponseWriter{
				Writer:         resBody,
				ResponseWriter: c.Response().Writer,
			}
			c.Response().Writer = writer

			// Call next handler
			err := next(c)

			// Determine log level and message
			statusCode := c.Response().Status
			var level, message string
			if err != nil || statusCode >= 400 {
				level = "ERROR"
				message = "error"
			} else {
				level = "INFO"
				message = "success"
			}

			var levelColor string
			if level == "ERROR" {
				levelColor = ColorRed
			} else if level == "INFO" {
				levelColor = ColorGreen
			} else {
				levelColor = ColorYellow
			}

			fmt.Printf("[%s %s%s%s requestId=%s, statusCode=%s-%s, method=%s, path=%s, requestBody=%s, responseBody=%s]\n",
				start.Format("2006-01-02 15:04:05"),
				levelColor, level, ColorReset,
				requestID,
				http.StatusText(statusCode),
				message,
				c.Request().Method,
				path,
				bodyStr,
				resBody.String(),
			)

			return err
		}
	}
}
