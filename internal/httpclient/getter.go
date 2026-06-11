package httpclient

import "context"

// Getter is the read-side contract consumers (scrapers, collectors) depend on.
// Declared here so multiple consumer packages can share the same shape instead
// of each re-declaring an identical one-method interface. The concrete *Client
// satisfies it; tests can stub it without spinning a real HTTP server.
type Getter interface {
	Get(ctx context.Context, url string) ([]byte, error)
}

// Compile-time check: the concrete polite client must satisfy the consumer
// contract.
var _ Getter = (*Client)(nil)
