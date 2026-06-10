package httpclient

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"net/url"
	"sync"
	"time"

	"golang.org/x/time/rate"
)

const (
	userAgent      = "Ottawa-WaitTime-Bot/1.0 (+https://github.com/adrien2121/GoProject)"
	jitterFraction = 0.20
)

// Client is a polite HTTP client with per-hostname rate limiting.
// Scrapers hitting the same hostname are queued one at a time (serialized); different hostnames run in parallel.
// Intended for a small, bounded set of hostnames (the list of hospital sites): the limiter map
// grows unboundedly with hostname diversity, so do not reuse this for general crawling.
type Client struct {
	http *http.Client
	// mu guards limiters.
	mu sync.Mutex
	// limiters keyed by hostname (e.g. "ottawahospital.on.ca").
	limiters    map[string]*rate.Limiter
	intervalSec int
}

// New returns a Client with no client-level timeout (callers control deadlines via ctx)
// and the given per-domain rate limit. intervalSec is the minimum seconds between requests
// to the same domain.
func New(intervalSec int) *Client {
	return &Client{
		http:        &http.Client{}, // deadlines come from per-request ctx, not http.Client.Timeout
		limiters:    make(map[string]*rate.Limiter),
		intervalSec: intervalSec,
	}
}

// Get performs a rate-limited GET request. Blocks until the per-domain limiter allows.
// Apply a per-request deadline by passing a context.WithTimeout.
func (c *Client) Get(ctx context.Context, rawURL string) ([]byte, error) {
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return nil, fmt.Errorf("httpclient.Get parse url: %w", err)
	}

	limiter := c.limiterFor(parsed.Host)

	// random delay to stagger concurrent scrapers
	//nolint:gosec // G404: scheduling jitter, not crypto
	jitter := time.Duration(float64(time.Duration(c.intervalSec)*time.Second) * jitterFraction * rand.Float64())
	// select will wait until either context cancelled or jitter elapsed
	select {
	// context cancelled (shutdown/timeout). Bail immediately
	case <-ctx.Done():
		return nil, fmt.Errorf("rate limiter: %w", ctx.Err())
	// jitter elapsed. Proceed. Select used instead of time.Sleep so cancellation works
	case <-time.After(jitter):
	}

	// block until token bucket allows next request for this domain
	if err := limiter.Wait(ctx); err != nil {
		return nil, fmt.Errorf("httpclient.Get rate limiter: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("httpclient.Get build request: %w", err)
	}
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("httpclient.Get %s: %w", rawURL, err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("httpclient.Get %s: unexpected status %d", rawURL, resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("httpclient.Get read body %s: %w", rawURL, err)
	}
	return body, nil
}

// limiterFor returns (or creates) the rate.Limiter for a given host.
func (c *Client) limiterFor(host string) *rate.Limiter {
	c.mu.Lock()
	defer c.mu.Unlock()
	if l, ok := c.limiters[host]; ok {
		return l
	}
	// burst=1: no bursting, strictly one req per interval
	l := rate.NewLimiter(rate.Every(time.Duration(c.intervalSec)*time.Second), 1)
	c.limiters[host] = l
	return l
}
