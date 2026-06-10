package sites

import "context"

// HTTPGetter is the subset of httpclient.Client that scrapers in this package
// depend on. Declared at the consumer side so scrapers couple to a behavior,
// not to httpclient's concrete struct: tests can stub it without spinning a
// real HTTP server, and a future swap of the polite client (caching, replay,
// per-host retry) does not require touching the scrapers.
type HTTPGetter interface {
	Get(ctx context.Context, url string) ([]byte, error)
}
