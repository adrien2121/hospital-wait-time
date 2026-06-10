//go:build integration

// Endpoint family: hospital reads (/api/v1/hospitals, /api/v1/hospitals/{id}).
// The hospitals visible to this endpoint come from migrations/000003_seed_hospitals
// — the same seed production runs. No test-only Upsert needed; the list is what
// it would be on a freshly migrated production DB.

package main

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

func TestEndpoint_ListHospitals_ReturnsSeededActiveRows(t *testing.T) {
	t.Run(`
		given the seed migration ran on a fresh Postgres,
		when a client GETs /api/v1/hospitals,
		then the response is 200 with the seed's active hospitals (cheo and montfort)
		and inactive ones (toh-civic, toh-general, qch) are filtered out`,
		func(t *testing.T) {
			// Given: full stack with the seed already applied by migrations.
			url, _, cleanup := startStack(t)
			defer cleanup()

			// When: a real HTTP client hits the list endpoint.
			resp, err := http.Get(url + "/api/v1/hospitals")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then:
			// 1. HTTP status is 200 — the handler executed, no server error.
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}

			// 2. Body decodes as a JSON array of objects. If decode fails the API
			//    shape regressed (e.g. someone returned an object instead of an array).
			body, _ := io.ReadAll(resp.Body)
			var got []map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode: %v (body=%q)", err, string(body))
			}

			// 3. Every row carries active=true. The endpoint must filter inactive rows
			//    server-side so the public API never advertises a closed facility.
			ids := map[string]bool{}
			for _, h := range got {
				ids[h["id"].(string)] = true
				if h["active"] != true {
					t.Errorf("row %v has active=false; the endpoint must filter these out", h)
				}
			}

			// 4. The seeded active hospitals (cheo, montfort) appear in the response —
			//    confirms the seed migration ran AND the SQL WHERE active=TRUE matches them.
			for _, want := range []string{"cheo", "montfort"} {
				if !ids[want] {
					t.Errorf("missing seeded active hospital %q in %v", want, ids)
				}
			}

			// 5. The seeded inactive hospitals (toh-civic, toh-general, qch) DO NOT
			//    appear — guards against a regression that drops the active=TRUE filter.
			for _, mustNot := range []string{"toh-civic", "toh-general", "qch"} {
				if ids[mustNot] {
					t.Errorf("inactive seeded hospital %q leaked into response", mustNot)
				}
			}
		},
	)
}

func TestEndpoint_GetHospital_200ForSeededID(t *testing.T) {
	t.Run(`
		given the seed migration loaded 'cheo' as an active hospital,
		when a client GETs /api/v1/hospitals/cheo,
		then the response is 200 with cheo's row (snake_case keys)`,
		func(t *testing.T) {
			// Given: full stack with seed applied.
			url, _, cleanup := startStack(t)
			defer cleanup()

			// When: a real HTTP client asks for the seeded hospital by ID.
			resp, err := http.Get(url + "/api/v1/hospitals/cheo")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then:
			// 1. HTTP status is 200 — the path param routed correctly and the row was found.
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d, want 200", resp.StatusCode)
			}

			// 2. Body decodes as a single JSON object (not an array) — confirms the
			//    handler returned one row, not the list shape.
			body, _ := io.ReadAll(resp.Body)
			var got map[string]any
			if err := json.Unmarshal(body, &got); err != nil {
				t.Fatalf("decode: %v (body=%q)", err, string(body))
			}

			// 3. The 'id' field comes back as 'cheo' — confirms the path param 'cheo'
			//    was passed through to the repo SQL and the matching row returned.
			if got["id"] != "cheo" {
				t.Errorf("id = %v, want cheo", got["id"])
			}

			// 4. JSON key uses snake_case ('facility_type', not 'FacilityType') and
			//    value matches what the seed migration set ('er'). Guards against a
			//    regression in the toHospitalResponse mapper or the seed data.
			if got["facility_type"] != "er" {
				t.Errorf("facility_type = %v, want er", got["facility_type"])
			}
		},
	)
}

func TestEndpoint_GetHospital_404WhenMissing(t *testing.T) {
	t.Run(`
		given an ID neither seeded nor inserted by any test,
		when a client GETs /api/v1/hospitals/{id},
		then the response is 404 because the repository returns ErrNotFound and the handler maps it`,
		func(t *testing.T) {
			// Given: full stack, no extra rows seeded by the test.
			url, _, cleanup := startStack(t)
			defer cleanup()

			// When: a real HTTP client asks for an unknown hospital.
			resp, err := http.Get(url + "/api/v1/hospitals/no-such-id")
			if err != nil {
				t.Fatalf("GET: %v", err)
			}
			defer resp.Body.Close()

			// Then: 404 — confirms the full chain (repo → service → handler) maps
			// the ErrNotFound sentinel to the right HTTP status.
			if resp.StatusCode != http.StatusNotFound {
				t.Fatalf("status = %d, want 404", resp.StatusCode)
			}
		},
	)
}
