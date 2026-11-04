import { withCSRF } from '#lib/shared/api/client.js';

/** The OTLP export route, proxied to the collector by the Go API (ADR 0002). */
const TRACES_PATH = '/api/v1/traces';
const API_PATH_PREFIX = '/api/';
const SERVICE_NAME = 'pvmss-web';
const SPAN_KIND_INTERNAL = 1;
const NANOS_PER_MS = 1e6;
/** 'event' entries below this duration are not dispatched by the observer. */
const EVENT_DURATION_THRESHOLD_MS = 16;

/** Accumulated web vitals for the current document, milliseconds (CLS unitless). */
export interface WebVitals {
	lcp: number;
	inp: number;
	cls: number;
	fcp: number;
	ttfb: number;
}

interface OtlpAnyValue {
	stringValue?: string;
	doubleValue?: number;
}

interface OtlpKeyValue {
	key: string;
	value: OtlpAnyValue;
}

interface OtlpSpan {
	traceId: string;
	spanId: string;
	name: string;
	kind: number;
	startTimeUnixNano: string;
	endTimeUnixNano: string;
	attributes: OtlpKeyValue[];
}

export interface OtlpExport {
	resourceSpans: {
		resource: { attributes: OtlpKeyValue[] };
		scopeSpans: { scope: { name: string }; spans: OtlpSpan[] }[];
	}[];
}

export interface PageviewSpanInput {
	traceId: string;
	spanId: string;
	startNanos: number;
	endNanos: number;
	vitals: WebVitals;
	path: string;
}

export interface VitalsCollector {
	record(entries: PerformanceEntry[]): void;
	snapshot(): WebVitals;
}

interface LayoutShiftEntry extends PerformanceEntry {
	hadRecentInput: boolean;
	value: number;
}

/**
 * Best-effort web-vitals accumulator. INP is approximated by the longest
 * 'event' entry duration (the real metric is a high percentile of
 * interactions; a single page session rarely has enough for the difference
 * to matter). CLS sums layout shifts that did not follow user input.
 */
export function createVitalsCollector(): VitalsCollector {
	const vitals: WebVitals = { lcp: 0, inp: 0, cls: 0, fcp: 0, ttfb: 0 };
	return {
		record(entries: PerformanceEntry[]): void {
			for (const entry of entries) {
				switch (entry.entryType) {
					case 'largest-contentful-paint':
						vitals.lcp = entry.startTime;
						break;
					case 'layout-shift': {
						const shift = entry as LayoutShiftEntry;
						if (!shift.hadRecentInput) vitals.cls += shift.value;
						break;
					}
					case 'event':
						vitals.inp = Math.max(vitals.inp, entry.duration);
						break;
					case 'paint':
						if (entry.name === 'first-contentful-paint') vitals.fcp = entry.startTime;
						break;
					case 'navigation':
						vitals.ttfb = (entry as PerformanceNavigationTiming).responseStart;
						break;
				}
			}
		},
		snapshot(): WebVitals {
			return { ...vitals };
		}
	};
}

/** Builds the OTLP/HTTP JSON payload for one browser.pageview span. */
export function buildPageviewExport(input: PageviewSpanInput): OtlpExport {
	const attributes: OtlpKeyValue[] = [{ key: 'url.path', value: { stringValue: input.path } }];
	const vitalKeys = { lcp: 'web_vitals.lcp', inp: 'web_vitals.inp', cls: 'web_vitals.cls', fcp: 'web_vitals.fcp', ttfb: 'web_vitals.ttfb' } as const;
	for (const [vital, key] of Object.entries(vitalKeys)) {
		const value = input.vitals[vital as keyof WebVitals];
		if (value > 0) attributes.push({ key, value: { doubleValue: value } });
	}
	return {
		resourceSpans: [
			{
				resource: { attributes: [{ key: 'service.name', value: { stringValue: SERVICE_NAME } }] },
				scopeSpans: [
					{
						scope: { name: SERVICE_NAME },
						spans: [
							{
								traceId: input.traceId,
								spanId: input.spanId,
								name: 'browser.pageview',
								kind: SPAN_KIND_INTERNAL,
								startTimeUnixNano: String(Math.round(input.startNanos)),
								endTimeUnixNano: String(Math.round(input.endNanos)),
								attributes
							}
						]
					}
				]
			}
		]
	};
}

let installed = false;

/**
 * Turns browser telemetry on when the operator configured a collector: the
 * public version endpoint reports `browserTelemetry`, which is the only
 * switch (ADR 0002). When on, same-origin API calls carry a `traceparent`
 * header so server spans join the page-view trace, and one pageview span with
 * the web vitals is exported on pagehide. Never throws - telemetry must not
 * break the app that runs it.
 */
export function installBrowserTelemetry(): Promise<void> {
	if (installed || globalThis.window === undefined) return Promise.resolve();
	installed = true;
	return activate().catch(() => {
		// A refused or failed flag fetch leaves telemetry off, by design.
	});
}

async function activate(): Promise<void> {
	const response = await globalThis.fetch('/api/v1/public/version', { headers: { Accept: 'application/json' } });
	if (!response.ok) return;
	const info = (await response.json()) as { browserTelemetry?: boolean };
	if (info.browserTelemetry !== true) return;

	const traceId = randomHex(16);
	const spanId = randomHex(8);
	wrapFetchWithTraceparent(`00-${traceId}-${spanId}-01`);

	const vitals = createVitalsCollector();
	// The observers must stay reachable: an unreferenced PerformanceObserver
	// can be garbage-collected and silently stop delivering entries.
	const observers = observeVitals(vitals);

	const startNanos = Math.round(globalThis.performance.timeOrigin * NANOS_PER_MS);
	globalThis.addEventListener('pagehide', () => {
		// Entries queued but not yet delivered would be dropped by disconnect;
		// takeRecords flushes them into the collector first.
		for (const observer of observers) {
			vitals.record(observer.takeRecords());
			observer.disconnect();
		}
		const body = JSON.stringify(
			buildPageviewExport({
				traceId,
				spanId,
				startNanos,
				endNanos: Date.now() * NANOS_PER_MS,
				vitals: vitals.snapshot(),
				path: globalThis.location.pathname
			})
		);
		// keepalive survives the unload; sendBeacon cannot set the CSRF header.
		void globalThis
			.fetch(TRACES_PATH, withCSRF({ method: 'POST', keepalive: true, headers: { 'Content-Type': 'application/json' }, body }))
			.catch(() => {
				// Fire-and-forget: a dropped export on unload is acceptable.
			});
	});
}

function randomHex(byteLength: number): string {
	const bytes = new Uint8Array(byteLength);
	globalThis.crypto.getRandomValues(bytes);
	return Array.from(bytes, (b) => b.toString(16).padStart(2, '0')).join('');
}

/** Adds a W3C traceparent to same-origin /api/ calls so the server span joins
 *  the page-view trace as a child of the pageview span. Other origins and
 *  non-API paths are passed through untouched. */
function wrapFetchWithTraceparent(traceparent: string): void {
	const original = globalThis.fetch;
	globalThis.fetch = (input: RequestInfo | URL, init?: RequestInit): Promise<Response> => {
		const raw = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
		if (!isSameOriginAPIPath(raw)) return original(input, init);
		const headers = new Headers(input instanceof Request ? input.headers : undefined);
		new Headers(init?.headers).forEach((value, key) => headers.set(key, value));
		headers.set('traceparent', traceparent);
		return original(input, { ...init, headers });
	};
}

function isSameOriginAPIPath(raw: string): boolean {
	try {
		const url = new URL(raw, globalThis.location.origin);
		return url.origin === globalThis.location.origin && url.pathname.startsWith(API_PATH_PREFIX);
	} catch {
		return false;
	}
}

function observeVitals(collector: VitalsCollector): PerformanceObserver[] {
	const observers: PerformanceObserver[] = [];
	if (typeof PerformanceObserver === 'undefined') return observers;
	const observe = (type: string, extra: { durationThreshold?: number } = {}): void => {
		try {
			const observer = new PerformanceObserver((list) => collector.record(list.getEntries()));
			observer.observe({ type, buffered: true, ...extra });
			observers.push(observer);
		} catch {
			// Safari rejects types it does not know; vitals are best-effort.
		}
	};
	observe('largest-contentful-paint');
	observe('layout-shift');
	observe('paint');
	observe('navigation');
	observe('event', { durationThreshold: EVENT_DURATION_THRESHOLD_MS });
	return observers;
}
