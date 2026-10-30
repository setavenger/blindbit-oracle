package indexer

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
)

// Bitcoin Core is regularly unavailable for a while: it answers REST with 503
// until it has loaded its block index, it refuses connections while it
// restarts, and a busy node can let a request time out. The oracle waits for
// it instead of exiting: requests that fail this way are retried with a capped
// backoff until Core answers or the context is cancelled. Answers that waiting
// cannot change (404 from a node without REST, 401 from a proxy, a body that
// does not decode, an invalid endpoint URL, a TLS mismatch) are returned at
// once, so a misconfiguration still fails fast.
//
// Variables so tests can shorten them.
var (
	retryInitialDelay = 500 * time.Millisecond
	retryMaxDelay     = 15 * time.Second
	// coreWaitLogInterval is how often a warning is repeated while Core stays
	// unavailable. Single failed attempts are only logged at debug level.
	coreWaitLogInterval = time.Minute
)

// coreUnavailableError marks a failure that waiting for Bitcoin Core can fix.
type coreUnavailableError struct{ err error }

func (e *coreUnavailableError) Error() string { return e.err.Error() }
func (e *coreUnavailableError) Unwrap() error { return e.err }

func isCoreUnavailable(err error) bool {
	var unavailable *coreUnavailableError
	return errors.As(err, &unavailable)
}

// coreStatusError is a non-200 answer from Bitcoin Core's REST interface.
type coreStatusError struct {
	url    string
	status string // e.g. "404 Not Found"
	code   int
	detail string // start of the body, where Core puts its reason
}

func (e *coreStatusError) Error() string {
	msg := fmt.Sprintf("bitcoin core REST %s answered %s", e.url, e.status)
	if e.detail != "" {
		msg += fmt.Sprintf(": %q", e.detail)
	}
	switch {
	case e.code == http.StatusUnauthorized || e.code == http.StatusForbidden:
		msg += " (Bitcoin Core's REST interface takes no credentials: check core_rest_endpoint and any proxy in front of the node)"
	case e.code == http.StatusNotFound && e.detail == "":
		// Core answers an unregistered path, i.e. REST switched off, with an
		// empty 404. Its own 404s (unknown block, height out of range) say why.
		msg += " (is bitcoind running with rest=1, and does core_rest_endpoint point at it?)"
	}
	return msg
}

// retryableStatus reports whether Core (or a proxy in front of it) is only
// temporarily unable to answer. Core uses 503 while it starts, while it shuts
// down and when its RPC work queue is full.
func retryableStatus(code int) bool {
	switch code {
	case http.StatusServiceUnavailable, http.StatusBadGateway,
		http.StatusGatewayTimeout, http.StatusTooManyRequests:
		return true
	}
	return false
}

// isEndpointMismatch reports transport failures that waiting cannot fix: the
// endpoint answers plain HTTP to an https:// URL (or the reverse), or presents
// a TLS certificate that is not trusted. Every other transport failure
// (refused, reset or dropped connections, timeouts, name lookups, which for a
// container's hostname succeed only once the container runs) is treated as
// Core being unavailable: exiting on those is what this file exists to avoid.
func isEndpointMismatch(err error) bool {
	var recordErr tls.RecordHeaderError
	var certErr *tls.CertificateVerificationError
	var authorityErr x509.UnknownAuthorityError
	var hostnameErr x509.HostnameError
	var invalidErr x509.CertificateInvalidError
	return errors.Is(err, http.ErrSchemeMismatch) ||
		errors.As(err, &recordErr) || errors.As(err, &certErr) ||
		errors.As(err, &authorityErr) || errors.As(err, &hostnameErr) ||
		errors.As(err, &invalidErr)
}

// bodyReader records a failed read of the response body, so that a connection
// lost mid-response (worth retrying) can be told apart from a complete
// response that does not decode (not worth retrying).
type bodyReader struct {
	r   io.Reader
	err error
}

func (b *bodyReader) Read(p []byte) (int, error) {
	n, err := b.r.Read(p)
	if err != nil && err != io.EOF {
		b.err = err
	}
	return n, err
}

// restGet fetches path from Bitcoin Core's REST interface and decodes the
// answer, waiting for Core while it is unavailable. It returns when the request
// succeeds, fails in a way that waiting cannot fix, or ctx is done.
func restGet[T any](ctx context.Context, path string, decode func(io.Reader) (T, error)) (T, error) {
	delay := retryInitialDelay
	for {
		v, err := restGetOnce(ctx, path, decode)
		if err == nil {
			coreAvailability.up()
			return v, nil
		}
		if ctx.Err() != nil {
			return v, withCtxErr(ctx, err)
		}
		if !isCoreUnavailable(err) {
			return v, err
		}

		// Jitter spreads out the parallel block requests that all failed at once.
		wait := delay/2 + rand.N(delay/2+1)
		coreAvailability.down(path, err, wait)

		timer := time.NewTimer(wait)
		select {
		case <-ctx.Done():
			timer.Stop()
			return v, withCtxErr(ctx, err)
		case <-timer.C:
		}
		delay = min(2*delay, retryMaxDelay)
	}
}

// withCtxErr makes sure an error returned because ctx is done matches ctx.Err().
func withCtxErr(ctx context.Context, err error) error {
	if errors.Is(err, ctx.Err()) {
		return err
	}
	return fmt.Errorf("%w (last error: %v)", ctx.Err(), err)
}

// restGetOnce makes a single request. Failures that waiting can fix are
// wrapped in coreUnavailableError.
func restGetOnce[T any](ctx context.Context, path string, decode func(io.Reader) (T, error)) (T, error) {
	var zero T
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, config.RestEndpoint+path, nil)
	if err != nil {
		return zero, fmt.Errorf("error creating request (check core_rest_endpoint): %w", err)
	}
	if (req.URL.Scheme != "http" && req.URL.Scheme != "https") || req.URL.Host == "" {
		return zero, fmt.Errorf("core_rest_endpoint %q is not an http:// or https:// URL", config.RestEndpoint)
	}

	resp, err := httpClient.Do(req) // <-- reuse the shared client
	if err != nil {
		err = fmt.Errorf("error performing request: %w", err)
		if isEndpointMismatch(err) {
			return zero, err
		}
		return zero, &coreUnavailableError{err}
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		detail, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		err := &coreStatusError{
			url:    req.URL.Redacted(),
			status: resp.Status,
			code:   resp.StatusCode,
			detail: strings.TrimSpace(string(detail)),
		}
		if retryableStatus(resp.StatusCode) {
			return zero, &coreUnavailableError{err}
		}
		return zero, err
	}

	body := &bodyReader{r: resp.Body}
	v, err := decode(body)
	if err != nil {
		if body.err != nil {
			return zero, &coreUnavailableError{fmt.Errorf("reading %s: %w", path, body.err)}
		}
		return zero, fmt.Errorf("decoding %s: %w", path, err)
	}
	return v, nil
}

// coreAvailability logs when Bitcoin Core becomes unavailable and when it is
// back, once per outage rather than once per request: during a sync up to
// MaxParallelRequests requests wait for it at the same time.
var coreAvailability coreState

type coreState struct {
	unavailable atomic.Bool // read on every successful request

	mu      sync.Mutex
	since   time.Time
	lastLog time.Time
}

func (c *coreState) down(path string, err error, retryIn time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()

	now := time.Now()
	switch {
	case !c.unavailable.Load():
		c.unavailable.Store(true)
		c.since, c.lastLog = now, now
		logging.L.Warn().Err(err).
			Str("endpoint", redactedEndpoint()).
			Str("request", path).
			Str("retry_in", retryIn.Round(time.Millisecond).String()).
			Msg("bitcoin core is unavailable or still starting; indexing waits for it and retries")
	case now.Sub(c.lastLog) >= coreWaitLogInterval:
		c.lastLog = now
		logging.L.Warn().Err(err).
			Str("endpoint", redactedEndpoint()).
			Str("waiting_for", now.Sub(c.since).Round(time.Second).String()).
			Msg("still waiting for bitcoin core")
	default:
		logging.L.Debug().Err(err).
			Str("request", path).
			Str("retry_in", retryIn.Round(time.Millisecond).String()).
			Msg("bitcoin core request failed; retrying")
	}
}

func (c *coreState) up() {
	if !c.unavailable.Load() {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	if !c.unavailable.Load() {
		return
	}
	c.unavailable.Store(false)
	logging.L.Info().
		Str("endpoint", redactedEndpoint()).
		Str("unavailable_for", time.Since(c.since).Round(time.Second).String()).
		Msg("bitcoin core is available again; indexing resumes")
}

// redactedEndpoint is core_rest_endpoint without a password, for logs.
func redactedEndpoint() string {
	if u, err := url.Parse(config.RestEndpoint); err == nil {
		return u.Redacted()
	}
	return config.RestEndpoint
}
