package collector

import "context"

// HTTPGetter is the subset of httpclient.Client that collectors in this package
// depend on. Declared at the consumer side so collectors couple to a behavior,
// not to httpclient's concrete struct: tests can stub it (or point a real client
// at an httptest.Server) without touching the polite-client implementation.
type HTTPGetter interface {
	Get(ctx context.Context, url string) ([]byte, error)
}
