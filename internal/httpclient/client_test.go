package httpclient

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGet_TargetReturning200(t *testing.T) {
	t.Run(`
		given a target hospital site returning 200 OK,
		when a scraper calls Get,
		then it receives the response body and our bot User-Agent is attached`,
		func(t *testing.T) {
			// Given: a fake site that returns 200 and asserts the User-Agent on its way in.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if got := r.Header.Get("User-Agent"); !strings.HasPrefix(got, "Ottawa-WaitTime-Bot/") {
					t.Errorf("missing or wrong User-Agent: %q", got)
				}
				_, _ = w.Write([]byte("hello"))
			}))
			defer srv.Close()

			c := New(0) // intervalSec=0 skips the rate limiter delay so the test stays fast.
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// When: the scraper calls Get against the fake site.
			body, err := c.Get(ctx, srv.URL)

			// Then: no error, body matches.
			if err != nil {
				t.Fatalf("Get: %v", err)
			}
			if string(body) != "hello" {
				t.Fatalf("body = %q, want hello", body)
			}
		},
	)
}

func TestGet_TargetReturning500(t *testing.T) {
	t.Run(`
		given a target hospital site returning 500 Internal Server Error,
		when a scraper calls Get,
		then it returns an error the orchestrator can count as a failure`,
		func(t *testing.T) {
			// Given: a fake site that always returns 500.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.WriteHeader(http.StatusInternalServerError)
			}))
			defer srv.Close()

			c := New(0)
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()

			// When: the scraper calls Get against the failing site.
			_, err := c.Get(ctx, srv.URL)

			// Then: the client surfaces a non-nil error.
			if err == nil {
				t.Fatal("expected error on 500 response")
			}
		},
	)
}

func TestGet_ContextCancelled(t *testing.T) {
	t.Run(`
		given the caller cancels its context before the request completes
		(e.g. the scraper receives SIGTERM mid-call),
		when a scraper calls Get,
		then Get returns a cancellation error promptly and does not block shutdown`,
		func(t *testing.T) {
			// Given: a deliberately slow fake site, and a context cancelled before Get is called.
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				time.Sleep(5 * time.Second) // never reached if cancellation works
				_, _ = w.Write([]byte("late"))
			}))
			defer srv.Close()

			c := New(0)
			ctx, cancel := context.WithCancel(context.Background())
			cancel() // pre-cancel before calling Get

			// When: the scraper calls Get with the already-cancelled context.
			_, err := c.Get(ctx, srv.URL)

			// Then: Get returns immediately with an error.
			if err == nil {
				t.Fatal("expected error on cancelled context")
			}
		},
	)
}

func TestLimiterFor_SameHostCached(t *testing.T) {
	t.Run(`
		given the same hostname is requested twice from the same Client,
		when two scrapers ask the client for its limiter,
		then they receive the same cached limiter and rate limiting is shared across them`,
		func(t *testing.T) {
			// Given: a Client and a fixed hostname.
			c := New(1)

			// When: two callers ask for the limiter of the same host.
			l1 := c.limiterFor("example.com")
			l2 := c.limiterFor("example.com")

			// Then: the second call returns the cached pointer.
			if l1 != l2 {
				t.Fatal("limiterFor returned different pointers for same host")
			}
		},
	)
}

func TestLimiterFor_DifferentHostsIsolated(t *testing.T) {
	t.Run(`
		given two different hostnames are requested,
		when scrapers ask the client for limiters,
		then each hostname gets its own limiter and a slow request to one hospital does not block another`,
		func(t *testing.T) {
			// Given: a Client and two distinct hostnames.
			c := New(1)

			// When: scrapers ask for the limiter of each.
			a := c.limiterFor("a.example.com")
			b := c.limiterFor("b.example.com")

			// Then: the two limiters are distinct objects.
			if a == b {
				t.Fatal("limiterFor returned same pointer for different hosts")
			}
		},
	)
}
