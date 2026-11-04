# 0002 - Browser telemetry: hand-rolled vitals and traceparent, proxied export

Status: accepted (implemented 2026-11-04)

## Context

The SPA already reports caught errors to `POST /api/v1/client-errors`
(`server/internal/httpapi/router.go:317`, handler in `client_errors.go`). The
project now runs an OTLP collector, so two browser-side signals are wanted:

- Web vitals (LCP, INP, CLS, FCP, TTFB) per page view.
- Trace context propagation, so a browser request and the server span it
  triggers share one trace.

The open questions were: is the OTel web SDK worth its bundle cost, how does
browser telemetry reach the collector without CORS, and does it need consent.

## Measurements

Built with `bun run build` in `web/` (bun 1.3.13, after
`bun install --frozen-lockfile`). Size is the sum over every `*.js` file in
`web/build`, raw bytes and `gzip -9` bytes:

| Variant                                                             | Files | Raw bytes                   | gzip -9 bytes            |
| ------------------------------------------------------------------- | ----: | --------------------------: | -----------------------: |
| Baseline (main at c9a60a31)                                         | 249   | 2,071,834                   | 641,581                  |
| OTel SDK, spec'd set, no context manager (headline)                 | 249   | 2,144,715 (+72,881, +3.5%)  | 664,891 (+23,310, +3.6%) |
| OTel SDK, spec'd set plus `ZoneContextManager` (context-zone 3.0.0) | 249   | 2,181,415 (+109,581, +5.3%) | 676,928 (+35,347, +5.5%) |
| Hand-rolled vitals + traceparent stub                               | 249   | 2,072,742 (+908, +0.04%)    | 642,073 (+492, +0.08%)   |

Measurement commands, run from the worktree root:

```bash
cd web && bun run build
find build -name '*.js' -exec sh -c 'printf "%s %s\n" "$(wc -c <"$1")" "$(gzip -9 -c "$1" | wc -c)"' _ {} \; \
  | awk '{r+=$1; g+=$2; n++} END{print n, r, g}'
```

The headline variant adds `@opentelemetry/api`, `sdk-trace-web` (2.12.0),
`instrumentation-fetch` and `exporter-trace-otlp-http` (0.300.0 each), and
`instrumentation` (0.300.0). `hooks.client.ts` then runs
`new WebTracerProvider({ spanProcessors: [new BatchSpanProcessor(new OTLPTraceExporter({ url: '/api/v1/traces' }))] })`,
`provider.register()` and `registerInstrumentations` with `FetchInstrumentation`
at module top level, so tree-shaking cannot drop them. The
`ZoneContextManager` row adds `@opentelemetry/context-zone` and passes it to
`register`. It is not required and is shown only to explain the larger number.
The hand-rolled variant is a roughly 60-line module (PerformanceObserver for
LCP, CLS, event and FCP, a `fetch` wrapper, a `sendBeacon` on `pagehide`).
Each variant was reverted with `git checkout` and `git clean -fd`, then
reinstalled. The post-revert build reproduced the baseline per file
(2,071,834 / 641,581, `git status` clean).

Per-file diff (baseline against the headline variant): the only file whose size
changed is the entry chunk `_app/immutable/entry/app.*.js`, 18,263 raw / 5,146
gzip before and 91,144 raw / 28,456 gzip after. The entry chunk loads on every
page, so the cost is paid on every page view, not only on traced ones.

Caveats: the web-vitals npm library was not measured (the stub is the
comparison, not the library). gzip -9 is used rather than the transport's
default level. The relative deltas are what matter.

## Server-side facts (verified in source, not by running)

- Global propagator: `server/internal/telemetry/telemetry.go:85`
  `otel.SetTextMapPropagator(TraceContext{}, Baggage{})`. It runs first in
  `Setup`, before its early return at `telemetry.go:90`. `main.go:76` calls
  `Setup` with no condition on telemetry config, so the global is set on every
  start.
- Explicit propagator: `server/internal/httpapi/tracing.go:22`
  `otelhttp.WithPropagators(TraceContext{}, Baggage{})`. The handler does not
  depend on the global; it joins an inbound `traceparent` even if
  `telemetry.Setup` never ran.
- Extraction: otelhttp v0.72.0 `handler.go:98`
  `h.propagators.Extract(r.Context(), HeaderCarrier(r.Header))`, and
  `config.go:107-113` shows `WithPropagators` replacing the default.
  So inbound `traceparent` is extracted. The default no-op propagator
  concern does not apply here.
- Coverage: `router.go:190` wraps the whole mux in `withTracing`. The filter
  `tracing.go:23` is `isAPIPath` (`security_headers.go:99-101`, prefix
  `/api`), so every `/api/v1/*` request gets a server span.
  Health and SPA assets are excluded.
- Gap: no test asserts extraction. `grep traceparent server/` finds nothing
  outside the otelhttp dependency. The existing `tracing_test.go` checks spans
  exist, not that the parent is joined.
- Self-trace risk: a new `/api/v1/traces` proxy would match `isAPIPath` and
  trace its own export. It needs an explicit exclusion.
- CORS: no `Access-Control-*` header exists anywhere in `server/`. The
  SPA is same-origin only.
- CSP: `security_headers.go:22` and `:37` set `connect-src 'self'`. A direct
  browser POST to a collector origin is blocked by CSP before CORS even
  applies, so a direct export would also need a CSP change.

## Decision

1. **Hand-rolled, no OTel web SDK.** Vitals come from `PerformanceObserver`.
   Trace propagation is a `fetch` wrapper that adds `traceparent` to
   same-origin `/api/` calls only. The SDK (spec'd set) adds 72.9 kB raw (+3.5%,
   +23.3 kB gzip) to the entry chunk on every page, against about 0.5 kB gzip
   for the stub. The SDK's
   automatic instrumentation (document load, XHR, long tasks) is not in the
   ask. If it becomes needed later, revisit with this table.
2. **Export proxied through the Go API.** Browser sends OTLP/HTTP JSON to
   `POST /api/v1/traces`, and the server forwards it to the configured
   collector endpoint. Same origin, so CSP `connect-src 'self'` holds and no
   CORS is added. Collector credentials (`OTEL_EXPORTER_OTLP_HEADERS`) stay
   server-side and never reach the browser. The route gets the same
   CSRF and per-IP rate-limit wrapping as `client-errors`.
3. **Consent: no per-user toggle, operator switch only.** The payload holds no
   user id, no cookie, no raw URL (route template only, as with
   client-errors), and no request or response body. Its IDs are random per
   page view. The only switch is operator-side: the feature is active only
   when a trace endpoint is configured, and the server tells the client so
   (see ticket 5). Revisit an admin toggle if a deployment's legal review
   requires an explicit notice.
4. **Web vitals reach the collector as spans**, not metrics. One
   `browser.pageview` span per page view, sent on `pagehide` with
   `fetch(..., { keepalive: true })` because the route's CSRF check needs the
   token header and `sendBeacon` cannot set headers. It carries `web_vitals.lcp`, `.inp`, `.cls`, `.fcp`, `.ttfb` as
   attributes. This reuses the one trace route. A histogram per vital needs a
   `/api/v1/metrics` proxy too, so it is deferred.

## Consequences

- Bundle cost is about 0.5 kB gzip, against 23.3 kB gzip for the spec'd SDK set
  (35.3 kB with ZoneContextManager).
- Hand-rolled code owns its correctness: INP and CLS edge cases (bfcache,
  `hadRecentInput`, interaction grouping) must be tested, not assumed. Use
  web-vitals only if those edge cases prove too costly to own.
- The browser must emit a span id that the server span can point to.
  Without one, server spans are orphans in the collector. The stub generates a
  span id per request but does not export it; this is ticket 3.
- Proxying makes the server forward untrusted bytes. The body must be capped
  and never logged (`logging rules: never log request bodies`).
- Propagation is only as good as the tests: ticket 2 adds the missing
  assertion.

## Rejected alternatives

- **OTel web SDK.** +3.5% raw and +3.6% gzip on every page (spec'd set;
  +5.3% and +5.5% with ZoneContextManager), for auto instrumentation this app
  does not need. It also does not measure web vitals
  itself, so the vitals library would be added on top.
- **Direct export to the collector with CORS.** Needs CORS headers on the
  collector, a CSP `connect-src` change in both policies, and leaks the
  collector origin into the browser. The proxy avoids all three.
- **Per-user consent toggle.** Adds UI and storage for data that carries no
  identity. The operator who configures the collector is the consent point.

## Implementation tickets (done 2026-11-04)

1. Server: `POST /api/v1/traces` proxy beside `client-errors` in
   `router.go`. Body cap (64 KiB), CSRF, per-IP rate limit (60/min), forward
   to the configured trace endpoint; not registered - 404 - when no endpoint
   is set. `/api/v1/traces` excluded from the `tracing.go` filter.
   (`traces_proxy.go`, `telemetry.TraceEndpoint`/`TraceHeaders`.)
2. Server test: `tracing_test.go` asserts the inbound `traceparent` is joined
   (same trace id, parent span id) and that the export route opens no span.
3. Web: `src/lib/shared/telemetry/browser-telemetry.ts` - vitals from
   `PerformanceObserver`, a `fetch` wrapper that adds `traceparent` for
   same-origin `/api/` calls, one `browser.pageview` span sent on `pagehide`
   with `keepalive`. Wired from `hooks.client.ts`; vitest tests cover the
   header, the payload shape and the opt-in flag.
4. Parent span: every API call carries the pageview span's id in
   `traceparent`, so server spans are children of the exported
   `browser.pageview` span.
5. Config flag: `GET /api/v1/public/version` reports `browserTelemetry`,
   true only when a trace endpoint is configured. Documented in
   `docs/configuration.md` and `docs/observability.md`.
6. Verified end to end against a stub collector: the proxy forwards the
   body verbatim with collector headers attached server-side, a real
   Chromium page emitted the pageview span on `pagehide`, and API calls
   carried a matching `traceparent`. Bundle delta: +3.0 kB raw / +1.3 kB
   gzip over the same baseline (stub: +0.9 / +0.5; SDK: +72.9 / +23.3).

## Not verified

- The web-vitals library's size was not measured (the stub is the comparison,
  not the library).
- The end-to-end run used a stub collector, not a production backend
  (Tempo, Jaeger). The proxy path is identical, but a real collector's
  rejection modes are untested.
- The measured hand-rolled cost (+3.0 kB raw / +1.3 kB gzip) is above the
  original stub estimate (+0.9 / +0.5) but remains far below the SDK set.
