package indexer

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/rs/zerolog"
	"github.com/setavenger/blindbit-lib/logging"
	"github.com/setavenger/blindbit-oracle/internal/config"
	"github.com/setavenger/blindbit-oracle/internal/database"
)

// These tests cover Bitcoin Core being unavailable: still starting (503),
// not listening yet (connection refused), and stopping and coming back while
// the indexer syncs or waits at the tip. Production incident 2026-10-17: after
// a VM reboot both oracles exited within a second of starting, because Core
// answered 503 (rest.go called log.Fatal) or refused the connection.

const (
	logCoreDown  = "bitcoin core is unavailable or still starting"
	logCoreUp    = "bitcoin core is available again"
	logCoreStill = "still waiting for bitcoin core"
)

// withFastRetry shortens the retry backoff and resets the availability state.
func withFastRetry(t *testing.T) {
	t.Helper()
	oldInitial, oldMax, oldLog := retryInitialDelay, retryMaxDelay, coreWaitLogInterval
	retryInitialDelay, retryMaxDelay = 2*time.Millisecond, 20*time.Millisecond
	coreAvailability.unavailable.Store(false)
	t.Cleanup(func() {
		retryInitialDelay, retryMaxDelay, coreWaitLogInterval = oldInitial, oldMax, oldLog
		coreAvailability.unavailable.Store(false)
	})
}

// withFastPolling makes ContinuousSync ask the node for new blocks often.
func withFastPolling(t *testing.T) {
	t.Helper()
	old := blockCheckInterval
	blockCheckInterval = 50 * time.Millisecond
	t.Cleanup(func() { blockCheckInterval = old })
}

func withSyncStartHeight(t *testing.T, height uint32) {
	t.Helper()
	old := config.SyncStartHeight
	config.SyncStartHeight = height
	t.Cleanup(func() { config.SyncStartHeight = old })
}

// logBuffer collects the indexer's log lines for one test.
type logBuffer struct {
	mu  sync.Mutex
	buf bytes.Buffer
}

func (l *logBuffer) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	return l.buf.Write(p)
}

func (l *logBuffer) count(msg string) int {
	l.mu.Lock()
	defer l.mu.Unlock()
	return strings.Count(l.buf.String(), msg)
}

func captureLogs(t *testing.T) *logBuffer {
	t.Helper()
	logs := &logBuffer{}
	old := logging.L
	logging.L = zerolog.New(logs).Level(zerolog.DebugLevel)
	t.Cleanup(func() { logging.L = old })
	return logs
}

// runIndexer runs the indexer the way `blindbit-oracle run` does
// (cmd/blindbit-oracle/main.go): integrity check, initial sync to the node's
// tip, then continuous sync until ctx is cancelled.
func runIndexer(ctx context.Context, b *Builder) error {
	if err := b.DBIntegrityCheck(ctx); err != nil {
		return fmt.Errorf("db integrity check failed: %w", err)
	}
	if err := b.InitialSyncToTip(ctx); err != nil {
		return fmt.Errorf("failed initial sync: %w", err)
	}
	if err := b.ContinuousSync(ctx); err != nil {
		return fmt.Errorf("continuous sync failed: %w", err)
	}
	return nil
}

// indexerRun is runIndexer running in the background.
type indexerRun struct {
	b      *Builder
	cancel context.CancelFunc
	done   chan struct{}
	err    error // set before done is closed
}

func startIndexer(t *testing.T, store *faultStore) *indexerRun {
	t.Helper()
	ctx, cancel := context.WithCancel(context.Background())
	r := &indexerRun{b: NewBuilder(ctx, store), cancel: cancel, done: make(chan struct{})}
	go func() {
		defer close(r.done)
		r.err = runIndexer(ctx, r.b)
	}()
	t.Cleanup(func() { _ = r.stop() })
	return r
}

// waitFor fails the test if the indexer exits, or cond does not hold within
// syncTimeout.
func (r *indexerRun) waitFor(t *testing.T, what string, cond func() bool) {
	t.Helper()
	deadline := time.Now().Add(syncTimeout)
	for !cond() {
		select {
		case <-r.done:
			t.Fatalf("indexer exited while waiting for %s: %v", what, r.err)
		default:
		}
		if time.Now().After(deadline) {
			t.Fatalf("timed out after %v waiting for %s", syncTimeout, what)
		}
		time.Sleep(5 * time.Millisecond)
	}
}

// stop cancels the indexer and returns its error.
func (r *indexerRun) stop() error {
	r.cancel()
	select {
	case <-r.done:
		return r.err
	case <-time.After(syncTimeout):
		return errDeadlock
	}
}

// assertIndexed checks that every height in [from, to] holds the node's block
// and that the integrity check finds no gap.
func assertIndexed(t *testing.T, b *Builder, node *faultNode, store *faultStore, from, to int) {
	t.Helper()
	for h := from; h <= to; h++ {
		want := node.hashAt(h)
		got, _ := store.GetBlockHashByHeight(uint32(h))
		if !bytes.Equal(got, want[:]) {
			t.Errorf("height %d: stored %x, node has %x", h, got, want[:])
		}
	}
	gaps, err := b.identifyGapRanges(uint32(from), uint32(to))
	if err != nil {
		t.Fatalf("identifyGapRanges: %v", err)
	}
	if len(gaps) > 0 {
		t.Errorf("integrity check found gaps: %+v", gaps)
	}
}

// Core answers 503 while it loads its block index. The oracle must wait for
// it instead of exiting, then sync. Before the fix: log.Fatal, process exit.
func TestIndexerWaitsForCoreWarmup(t *testing.T) {
	withParallelism(t, 2, 2)
	withSyncStartHeight(t, 0)
	withFastRetry(t)
	withFastPolling(t)
	logs := captureLogs(t)

	node := newFaultNode(t, 40)
	node.failAll(faultWarmup, 8)
	store := newFaultStore()
	run := startIndexer(t, store)

	run.waitFor(t, "all 40 blocks indexed", func() bool { return store.indexedCount() == 40 })
	if err := run.stop(); !errors.Is(err, context.Canceled) {
		t.Fatalf("indexer stopped with %v, want context.Canceled", err)
	}
	assertIndexed(t, run.b, node, store, 1, 40)
	if n := node.requestCount("chaininfo"); n < 9 {
		t.Errorf("node saw %d chaininfo requests, want the 8 answered with 503 and more", n)
	}
	if logs.count(logCoreDown) != 1 || logs.count(logCoreUp) != 1 {
		t.Errorf("want one %q and one %q line, got %d and %d",
			logCoreDown, logCoreUp, logs.count(logCoreDown), logs.count(logCoreUp))
	}
	assertNoIndexerGoroutines(t)
}

// Core is not listening yet when the oracle starts (connection refused).
// Before the fix: "failed to pull chainInfo", process exit.
func TestIndexerWaitsForCoreToListen(t *testing.T) {
	withParallelism(t, 2, 2)
	withSyncStartHeight(t, 0)
	withFastRetry(t)
	withFastPolling(t)
	logs := captureLogs(t)

	node := newOfflineFaultNode(t, 40)
	store := newFaultStore()
	run := startIndexer(t, store)

	run.waitFor(t, "the indexer to report Core unavailable", func() bool { return logs.count(logCoreDown) > 0 })
	time.Sleep(100 * time.Millisecond) // several refused attempts
	node.goOnline()

	run.waitFor(t, "all 40 blocks indexed", func() bool { return store.indexedCount() == 40 })
	if err := run.stop(); !errors.Is(err, context.Canceled) {
		t.Fatalf("indexer stopped with %v, want context.Canceled", err)
	}
	assertIndexed(t, run.b, node, store, 1, 40)
	if logs.count(logCoreUp) != 1 {
		t.Errorf("want one %q line, got %d", logCoreUp, logs.count(logCoreUp))
	}
	assertNoIndexerGoroutines(t)
}

// Core stops in the middle of the initial sync and comes back. In-flight
// requests fail, new ones are refused; the sync must resume without a restart,
// index every height exactly once and leave no gap.
func TestIndexerResumesAfterCoreRestartMidSync(t *testing.T) {
	withParallelism(t, 4, 2)
	withSyncStartHeight(t, 0)
	withFastRetry(t)
	withFastPolling(t)
	logs := captureLogs(t)

	const tip = 150
	node := newFaultNode(t, tip)
	store := newFaultStore()

	indexedAtOutage := make(chan int, 1)
	var once sync.Once
	node.mu.Lock()
	node.onRequest = func(endpoint string, height int) {
		if endpoint == "block" && height == tip/2 {
			once.Do(func() {
				go func() {
					node.goOffline()
					indexedAtOutage <- store.indexedCount()
					time.Sleep(300 * time.Millisecond)
					node.goOnline()
				}()
			})
		}
	}
	node.mu.Unlock()

	run := startIndexer(t, store)
	run.waitFor(t, "all blocks indexed", func() bool { return store.indexedCount() == tip })
	if err := run.stop(); !errors.Is(err, context.Canceled) {
		t.Fatalf("indexer stopped with %v, want context.Canceled", err)
	}

	select {
	case n := <-indexedAtOutage:
		if n == 0 || n >= tip {
			t.Errorf("%d of %d blocks were indexed when Core stopped, want an outage mid-sync", n, tip)
		}
	default:
		t.Fatalf("Core never went offline")
	}
	assertIndexed(t, run.b, node, store, 1, tip)
	for h := 1; h <= tip; h++ {
		if n := store.appliedAt(uint32(h)); n != 1 {
			t.Errorf("height %d applied %d times, want 1", h, n)
		}
	}
	if logs.count(logCoreDown) == 0 || logs.count(logCoreUp) == 0 {
		t.Errorf("want %q and %q lines, got %d and %d",
			logCoreDown, logCoreUp, logs.count(logCoreDown), logs.count(logCoreUp))
	}
	assertNoIndexerGoroutines(t)
}

// Core stops while the oracle is at the tip, blocks arrive while it is away,
// and it comes back. Continuous sync must pick up the new blocks.
func TestIndexerResumesAfterCoreRestartAtTip(t *testing.T) {
	withParallelism(t, 2, 2)
	withSyncStartHeight(t, 0)
	withFastRetry(t)
	withFastPolling(t)
	logs := captureLogs(t)

	node := newFaultNode(t, 20)
	store := newFaultStore()
	run := startIndexer(t, store)
	run.waitFor(t, "the initial sync", func() bool { return store.indexedCount() == 20 })

	node.goOffline()
	run.waitFor(t, "the indexer to report Core unavailable", func() bool { return logs.count(logCoreDown) > 0 })
	node.extend(5)
	time.Sleep(100 * time.Millisecond)
	node.goOnline()

	run.waitFor(t, "the blocks mined during the outage", func() bool { return store.indexedCount() == 25 })
	if err := run.stop(); !errors.Is(err, context.Canceled) {
		t.Fatalf("indexer stopped with %v, want context.Canceled", err)
	}
	assertIndexed(t, run.b, node, store, 1, 25)
	if logs.count(logCoreUp) == 0 {
		t.Errorf("no %q line", logCoreUp)
	}
	assertNoIndexerGoroutines(t)
}

// Answers that waiting cannot change must stop the oracle at once with a
// message that says what to fix, not retry forever.
func TestIndexerFailsFastOnConfigError(t *testing.T) {
	cases := []struct {
		name     string
		fault    faultKind
		endpoint string // overrides the node's URL when set
		want     []string
	}{
		{name: "proxy wants credentials", fault: faultUnauthorized,
			want: []string{"401 Unauthorized", "takes no credentials", "core_rest_endpoint"}},
		{name: "REST disabled", fault: faultRESTDisabled,
			want: []string{"404 Not Found", "rest=1"}},
		{name: "endpoint without scheme", endpoint: "127.0.0.1:8332",
			want: []string{"127.0.0.1:8332"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			withParallelism(t, 2, 2)
			withSyncStartHeight(t, 0)
			withFastRetry(t)
			logs := captureLogs(t)

			node := newFaultNode(t, 10)
			if tc.fault != 0 {
				node.failAll(tc.fault, -1)
			}
			if tc.endpoint != "" {
				config.RestEndpoint = tc.endpoint // restored by the node's cleanup
			}
			store := newFaultStore()
			b := NewBuilder(context.Background(), store)

			start := time.Now()
			err := returnsWithin(t, syncTimeout, "runIndexer", func() error {
				return runIndexer(context.Background(), b)
			})
			if errors.Is(err, errDeadlock) {
				return
			}
			if err == nil {
				t.Fatalf("runIndexer returned nil, want a configuration error")
			}
			if d := time.Since(start); d > time.Second {
				t.Errorf("runIndexer took %v to fail, want fail-fast", d)
			}
			for _, want := range tc.want {
				if !strings.Contains(err.Error(), want) {
					t.Errorf("error %q does not mention %q", err, want)
				}
			}
			if tc.fault != 0 {
				if n := node.requestCount("chaininfo"); n != 1 {
					t.Errorf("node saw %d chaininfo requests, want 1 (no retry)", n)
				}
			}
			if n := logs.count(logCoreDown); n != 0 {
				t.Errorf("logged %q %d times for a configuration error", logCoreDown, n)
			}
			if store.indexedCount() != 0 {
				t.Errorf("indexed %d blocks, want none", store.indexedCount())
			}
		})
	}
}

// Transient failures of single block requests, including a connection that
// breaks mid-body, are retried without failing the sync.
func TestSyncBlocksRetriesTransientErrors(t *testing.T) {
	withParallelism(t, 2, 2)
	withFastRetry(t)
	captureLogs(t)

	node := newFaultNode(t, 30)
	node.injectN("blockhashbyheight", 10, faultDrop, 2)
	node.injectN("block", 12, faultTruncate, 2)
	node.injectN("spenttxouts", 12, faultWarmup, 3)
	node.injectN("spenttxouts", 20, faultTruncate, 1)
	node.injectN("block", 25, faultDrop, 3)
	store := newFaultStore()
	b := NewBuilder(context.Background(), store)

	err := returnsWithin(t, syncTimeout, "SyncBlocks", func() error {
		return b.SyncBlocks(context.Background(), 1, 30)
	})
	if err != nil {
		t.Fatalf("SyncBlocks: %v", err)
	}
	assertIndexed(t, b, node, store, 1, 30)
	for h := 1; h <= 30; h++ {
		if n := store.appliedAt(uint32(h)); n != 1 {
			t.Errorf("height %d applied %d times, want 1", h, n)
		}
	}
	assertNoIndexerGoroutines(t)
}

// Many requests waiting for Core at once log one warning when it goes away,
// a reminder while it stays away, and one line when it is back.
func TestCoreAvailabilityLogging(t *testing.T) {
	withFastRetry(t)
	coreWaitLogInterval = 100 * time.Millisecond
	logs := captureLogs(t)
	node := newOfflineFaultNode(t, 5)

	var wg sync.WaitGroup
	errs := make([]error, 8)
	for i := range errs {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, errs[i] = getBlockHashByHeight(context.Background(), int64(i%5))
		}()
	}
	time.Sleep(350 * time.Millisecond)
	node.goOnline()
	wg.Wait()

	for i, err := range errs {
		if err != nil {
			t.Errorf("request %d: %v", i, err)
		}
	}
	if n := logs.count(logCoreDown); n != 1 {
		t.Errorf("%q logged %d times, want once", logCoreDown, n)
	}
	if n := logs.count(logCoreStill); n < 2 {
		t.Errorf("%q logged %d times, want a reminder every 100ms", logCoreStill, n)
	}
	if n := logs.count(logCoreUp); n != 1 {
		t.Errorf("%q logged %d times, want once", logCoreUp, n)
	}
}

// Waiting for Core ends when the caller's context does.
func TestRestGetStopsOnCancel(t *testing.T) {
	withFastRetry(t)
	captureLogs(t)
	newOfflineFaultNode(t, 5)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	start := time.Now()
	_, err := GetChainInfo(ctx)
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("GetChainInfo error = %v, want context.DeadlineExceeded", err)
	}
	if d := time.Since(start); d > time.Second {
		t.Errorf("GetChainInfo took %v to honour the deadline", d)
	}
}

// restGetOnce sorts failures into "wait for Core" and "give up".
func TestRestGetOnceClassifiesErrors(t *testing.T) {
	status := func(code int, body string) http.HandlerFunc {
		return func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(code)
			_, _ = io.WriteString(w, body)
		}
	}
	cases := []struct {
		name      string
		handler   http.HandlerFunc // nil: nothing listens
		endpoint  string           // overrides the server URL
		decode    string           // "hash" (32 raw bytes) or "json"
		tls       string           // "server": TLS server; "client": https:// URL to a plain server
		transient bool
	}{
		{name: "connection refused", decode: "hash", transient: true},
		{name: "503 warming up", handler: status(503, "Loading block index…"), decode: "json", transient: true},
		{name: "503 work queue full", handler: status(503, "Work queue depth exceeded"), decode: "json", transient: true},
		{name: "502 from proxy", handler: status(502, ""), decode: "json", transient: true},
		{name: "504 from proxy", handler: status(504, ""), decode: "json", transient: true},
		{name: "429 from proxy", handler: status(429, ""), decode: "json", transient: true},
		{name: "connection dropped", decode: "hash", transient: true, handler: func(w http.ResponseWriter, r *http.Request) {
			conn, _, _ := w.(http.Hijacker).Hijack()
			_ = conn.Close()
		}},
		{name: "body cut short", decode: "hash", transient: true, handler: func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Length", "32")
			_, _ = w.Write([]byte("short"))
		}},
		{name: "400 bad request", handler: status(400, "Invalid hash"), decode: "hash"},
		{name: "401 unauthorized", handler: status(401, ""), decode: "json"},
		{name: "403 forbidden", handler: status(403, ""), decode: "json"},
		{name: "404 REST disabled", handler: status(404, ""), decode: "json"},
		{name: "404 block pruned", handler: status(404, "abc not available (pruned data)"), decode: "hash"},
		{name: "500 internal error", handler: status(500, ""), decode: "json"},
		{name: "complete but short body", handler: status(200, "short"), decode: "hash"},
		{name: "complete but not JSON", handler: status(200, "<html>"), decode: "json"},
		{name: "no scheme", endpoint: "127.0.0.1:8332", decode: "json"},
		{name: "hostname without scheme", endpoint: "localhost:8332", decode: "json"},
		{name: "unsupported scheme", endpoint: "ftp://127.0.0.1:8332", decode: "json"},
		{name: "https to a plain HTTP node", tls: "client", handler: status(200, "{}"), decode: "json"},
		{name: "untrusted certificate", tls: "server", handler: status(200, "{}"), decode: "json"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			old := config.RestEndpoint
			t.Cleanup(func() { config.RestEndpoint = old })
			switch {
			case tc.endpoint != "":
				config.RestEndpoint = tc.endpoint
			case tc.handler == nil:
				ln, err := net.Listen("tcp", "127.0.0.1:0")
				if err != nil {
					t.Fatal(err)
				}
				config.RestEndpoint = "http://" + ln.Addr().String()
				_ = ln.Close()
			case tc.tls == "server":
				srv := httptest.NewTLSServer(tc.handler)
				t.Cleanup(srv.Close)
				config.RestEndpoint = srv.URL
			case tc.tls == "client":
				srv := httptest.NewServer(tc.handler)
				t.Cleanup(srv.Close)
				config.RestEndpoint = strings.Replace(srv.URL, "http://", "https://", 1)
			default:
				srv := httptest.NewServer(tc.handler)
				t.Cleanup(srv.Close)
				config.RestEndpoint = srv.URL
			}

			var err error
			if tc.decode == "hash" {
				_, err = restGetOnce(context.Background(), "/rest/blockhashbyheight/1.bin", decodeBlockHash)
			} else {
				_, err = restGetOnce(context.Background(), chainInfoPath, decodeChainInfo)
			}
			if err == nil {
				t.Fatalf("restGetOnce returned nil error")
			}
			if got := isCoreUnavailable(err); got != tc.transient {
				t.Errorf("isCoreUnavailable(%q) = %v, want %v", err, got, tc.transient)
			}
		})
	}
}

// A TLS mismatch between core_rest_endpoint and the node cannot go away by
// waiting; every other transport failure is treated as Core being away.
func TestIsEndpointMismatch(t *testing.T) {
	cases := []struct {
		err  error
		want bool
	}{
		{&url.Error{Op: "Get", URL: "u", Err: http.ErrSchemeMismatch}, true},
		{&url.Error{Op: "Get", URL: "u", Err: &tls.CertificateVerificationError{Err: x509.UnknownAuthorityError{}}}, true},
		{&url.Error{Op: "Get", URL: "u", Err: tls.RecordHeaderError{Msg: "first record does not look like a TLS handshake"}}, true},
		{&url.Error{Op: "Get", URL: "u", Err: &net.OpError{Op: "dial", Err: errors.New("connection refused")}}, false},
		{&url.Error{Op: "Get", URL: "u", Err: &net.DNSError{Err: "no such host", IsNotFound: true}}, false},
		{&url.Error{Op: "Get", URL: "u", Err: io.EOF}, false},
		{&url.Error{Op: "Get", URL: "u", Err: errors.New("http: server closed idle connection")}, false},
	}
	for _, tc := range cases {
		if got := isEndpointMismatch(tc.err); got != tc.want {
			t.Errorf("isEndpointMismatch(%v) = %v, want %v", tc.err, got, tc.want)
		}
	}
}

// roundTripFunc lets tests hold failed attempts until a newer request succeeds.
type roundTripFunc func(*http.Request) (*http.Response, error)

func (f roundTripFunc) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestCoreAvailabilityIgnoresStaleFailures(t *testing.T) {
	for _, recovering := range []bool{false, true} {
		t.Run(fmt.Sprintf("recovering=%v", recovering), func(t *testing.T) {
			withFastRetry(t)
			logs := captureLogs(t)
			coreAvailability.unavailable.Store(recovering)
			const callers = 8
			started := make(chan struct{}, callers)
			release := make(chan struct{})
			var attempts atomic.Int32
			oldClient, oldEndpoint := httpClient, config.RestEndpoint
			config.RestEndpoint = "http://core.test"
			ctx, cancel := context.WithTimeout(context.Background(), syncTimeout)
			defer cancel()
			httpClient = &http.Client{Transport: roundTripFunc(func(r *http.Request) (*http.Response, error) {
				if r.URL.Path == "/stale" && attempts.Add(1) <= callers {
					started <- struct{}{}
					select {
					case <-release:
					case <-r.Context().Done():
						return nil, r.Context().Err()
					}
					return nil, errors.New("failure from before the successful request")
				}
				return &http.Response{StatusCode: http.StatusOK, Body: io.NopCloser(strings.NewReader("{}")), Header: make(http.Header), Request: r}, nil
			})}
			t.Cleanup(func() { httpClient, config.RestEndpoint = oldClient, oldEndpoint })
			errs := make(chan error, callers)
			for i := 0; i < callers; i++ {
				go func() { _, err := restGet(ctx, "/stale", decodeChainInfo); errs <- err }()
			}
			for i := 0; i < callers; i++ {
				select {
				case <-started:
				case <-ctx.Done():
					t.Fatal("stale requests did not start")
				}
			}
			if _, err := GetChainInfo(ctx); err != nil {
				t.Fatal(err)
			}
			close(release)
			for i := 0; i < callers; i++ {
				if err := <-errs; err != nil {
					t.Fatal(err)
				}
			}
			if n := logs.count(logCoreDown); n != 0 {
				t.Errorf("stale failures logged %d new outages, want 0", n)
			}
			wantUp := 0
			if recovering {
				wantUp = 1
			}
			if n := logs.count(logCoreUp); n != wantUp {
				t.Errorf("recovery logged %d times, want %d", n, wantUp)
			}
			// A genuinely new failed attempt must still mark Core unavailable.
			outageCtx, stopOutage := context.WithTimeout(ctx, 30*time.Millisecond)
			defer stopOutage()
			_, err := restGet(outageCtx, "/new-outage", func(io.Reader) (*ChainInfo, error) { return nil, &coreUnavailableError{errors.New("new outage")} })
			if !errors.Is(err, context.DeadlineExceeded) {
				t.Fatalf("new outage: %v", err)
			}
			if n := logs.count(logCoreDown); n != 1 {
				t.Errorf("new outage logged %d times, want 1", n)
			}
		})
	}
}

type pollingFailStore struct {
	*faultStore
	node    *faultNode
	err     error
	sawPoll atomic.Bool
}

func (s *pollingFailStore) ApplyBlock(*database.DBBlock) error {
	before := s.node.requestCount("chaininfo")
	s.node.inject("chaininfo", 0, faultHang)
	deadline := time.Now().Add(syncTimeout / 2)
	for s.node.requestCount("chaininfo") == before {
		if time.Now().After(deadline) {
			return errors.New("poll never started")
		}
		time.Sleep(time.Millisecond)
	}
	s.sawPoll.Store(true)
	return s.err
}
func TestContinuousSyncWriterErrorWhilePolling(t *testing.T) {
	withParallelism(t, 1, 1)
	withSyncStartHeight(t, 0)
	withFastPolling(t)
	captureLogs(t)
	node := newFaultNode(t, 3)
	errWrite := errors.New("injected write failure while polling")
	store := &pollingFailStore{faultStore: newFaultStore(), node: node, err: errWrite}
	b := NewBuilder(context.Background(), store)
	err := returnsWithin(t, syncTimeout, "ContinuousSync", func() error { return b.ContinuousSync(context.Background()) })
	if !errors.Is(err, errWrite) {
		t.Fatalf("got %v, want %v", err, errWrite)
	}
	if !store.sawPoll.Load() {
		t.Fatal("writer did not overlap a poll")
	}
	assertNoIndexerGoroutines(t)
}

func TestSyncBlocksProgressReportUnavailable(t *testing.T) {
	for _, kind := range []faultKind{faultWarmup, faultHang} {
		t.Run(fmt.Sprintf("fault=%d", kind), func(t *testing.T) {
			withParallelism(t, 1, 1)
			withFastRetry(t)
			logs := captureLogs(t)
			oldInterval, oldTimeout := progressReportInterval, progressReportTimeout
			progressReportInterval, progressReportTimeout = time.Millisecond, 20*time.Millisecond
			t.Cleanup(func() { progressReportInterval, progressReportTimeout = oldInterval, oldTimeout })
			node := newFaultNode(t, 3)
			node.inject("chaininfo", 0, kind)
			reporting := make(chan struct{})
			var once sync.Once
			node.onRequest = func(endpoint string, height int) {
				if endpoint == "chaininfo" {
					once.Do(func() { close(reporting) })
				}
				if endpoint == "spenttxouts" && height == 1 {
					select {
					case <-reporting:
					case <-time.After(syncTimeout / 2):
					}
				}
			}
			b := NewBuilder(context.Background(), newFaultStore())
			if err := returnsWithin(t, syncTimeout, "SyncBlocks", func() error { return b.SyncBlocks(context.Background(), 1, 3) }); err != nil {
				t.Fatal(err)
			}
			if node.requestCount("chaininfo") == 0 {
				t.Fatal("progress report never requested chain info")
			}
			if n := logs.count("Indexer is up to date"); n != 0 {
				t.Errorf("unknown Core tip reported up to date %d times", n)
			}
			if n := logs.count("failed to get chain info for progress report"); n != 0 {
				t.Errorf("progress report emitted %d repetitive outage warnings", n)
			}
			assertNoIndexerGoroutines(t)
		})
	}
}
