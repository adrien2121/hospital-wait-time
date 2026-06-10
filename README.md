# Ottawa ER & Clinic Wait-Time Aggregator

Scrapes wait times from Ottawa-area hospitals, stores history in PostgreSQL, exposes a REST API for charts and analysis. Two binaries: `cmd/api` (HTTP) and `cmd/scraper` (daemon).

## Stack

- Go 1.26.4
- PostgreSQL 16
- `pgx/v5` (no ORM, explicit SQL)
- `goquery` for HTML scraping
- `golang.org/x/time/rate` for per-domain rate limiting
- `log/slog` for structured logging
- `golang-migrate` for schema migrations

## Prerequisites

- Go 1.26+
- Docker + Docker Compose (for local Postgres)
- `make`

## Local setup

```bash
cp .env.example .env          # adjust if needed
make up                       # starts Postgres
docker compose --profile tools run --rm migrate   # apply migrations
make run-api                  # in one shell
make run-scraper              # in another shell
```

The API listens on `http://localhost:8080`. See `internal/handler/*.go` for routes.

## Project layout

```
cmd/
  api/             REST server binary
  scraper/         scraper daemon binary
internal/
  bootstrap/       opens Postgres + builds repositories (shared by both binaries)
  config/          env-var -> Config struct
  domain/          entities, value objects, stable hospital IDs
  handler/         HTTP handlers + response DTOs
  httpclient/      polite rate-limited HTTP client
  logger/          slog JSON logger factory
  repository/      repo interfaces (consumer side)
  repository/postgres/    pgx-backed implementations
  scraper/         Scraper interface, Orchestrator, ParseWaitTime, BaseScraper
  scraper/sites/   per-hospital Scraper implementations
  service/         business logic (status, trend, anomaly, best-time)
migrations/        golang-migrate SQL files
```

## Environment

See `.env.example` for the full list. Required: `DATABASE_URL`. Optional everything else.

## Data sources

| Facility | Source |
|---|---|
| The Ottawa Hospital (Civic + General) | https://www.ottawahospital.on.ca/en/patients-visitors/emergency-wait-times/ |
| CHEO | https://www.cheo.on.ca/en/visiting-cheo/wait-times.aspx |
| Queensway Carleton Hospital | https://qch.ca/patients-visitors/emergency-department/ |
| Hôpital Montfort | https://www.hopitalmontfort.com/en/emergency |

## Polite scraping policy

- Custom `User-Agent` identifying the bot, with a contact URL.
- Default 1 request per 30s per domain, with ±20% jitter.
- Token-bucket limiter per hostname (different sites run in parallel; same site is serialised).
- After 3 consecutive failures the scraper backs off exponentially up to 1 hour.

## Make targets

Run `make help` for the full list. Common ones: `run-api`, `run-scraper`, `build`, `test`, `vet`, `lint`, `migrate-up`, `up`, `down`.
