import { afterEach, describe, expect, it, vi } from 'vitest';
import { buildPageviewExport, createVitalsCollector } from './browser-telemetry.js';

afterEach(() => vi.unstubAllGlobals());

// installBrowserTelemetry holds module state (installed once per page load),
// so each test re-imports a fresh copy after stubbing fetch.
async function freshModule() {
	vi.resetModules();
	return import('./browser-telemetry.js');
}

function flagResponse(browserTelemetry: boolean): Response {
	return new Response(JSON.stringify({ version: '0.4.0', browserTelemetry }), {
		status: 200,
		headers: { 'Content-Type': 'application/json' }
	});
}

function stubbedFetch(browserTelemetry = true): ReturnType<typeof vi.fn> {
	const fetchMock = vi.fn().mockImplementation((input: RequestInfo | URL) => {
		const url = typeof input === 'string' ? input : input instanceof URL ? input.href : input.url;
		if (url === '/api/v1/public/version') return Promise.resolve(flagResponse(browserTelemetry));
		return Promise.resolve(new Response(null, { status: 204 }));
	});
	vi.stubGlobal('fetch', fetchMock);
	return fetchMock;
}

function entry(partial: Record<string, unknown>): PerformanceEntry {
	return { name: '', entryType: '', startTime: 0, duration: 0, ...partial } as PerformanceEntry;
}

describe('installBrowserTelemetry', () => {
	it('adds a W3C traceparent to same-origin API calls when the server opts in', async () => {
		const fetchMock = stubbedFetch(true);
		const mod = await freshModule();
		await mod.installBrowserTelemetry();

		await globalThis.fetch('/api/v1/vms');
		const headers = (fetchMock.mock.calls.at(-1)?.[1] as RequestInit).headers as Headers;
		expect(headers.get('traceparent')).toMatch(/^00-[0-9a-f]{32}-[0-9a-f]{16}-01$/);
	});

	it('leaves non-API and cross-origin calls untouched', async () => {
		const fetchMock = stubbedFetch(true);
		const mod = await freshModule();
		await mod.installBrowserTelemetry();

		await globalThis.fetch('/about');
		await globalThis.fetch('https://collector.example/api/v1/traces');
		for (const call of fetchMock.mock.calls.slice(-2)) {
			expect((call[1] as RequestInit | undefined)?.headers).toBeUndefined();
		}
	});

	it('does not wrap fetch when the server reports telemetry off', async () => {
		const fetchMock = stubbedFetch(false);
		const mod = await freshModule();
		await mod.installBrowserTelemetry();

		expect(globalThis.fetch).toBe(fetchMock);
	});

	it('exports one browser.pageview span on pagehide, parent of the API calls', async () => {
		const fetchMock = stubbedFetch(true);
		const mod = await freshModule();
		await mod.installBrowserTelemetry();

		await globalThis.fetch('/api/v1/vms');
		const parentHeader = ((fetchMock.mock.calls.at(-1)?.[1] as RequestInit).headers as Headers).get('traceparent') ?? '';
		const [, traceId, spanId] = parentHeader.split('-');

		globalThis.dispatchEvent(new Event('pagehide'));

		// Listeners from earlier tests' module instances also answer pagehide;
		// the one to assert on is the export carrying this test's trace id.
		type ExportBody = {
			resourceSpans: { scopeSpans: { spans: { name: string; traceId: string; spanId: string; attributes: { key: string }[] }[] }[] }[];
		};
		const exports_ = fetchMock.mock.calls
			.filter((c) => c[0] === '/api/v1/traces')
			.map((c) => ({ init: c[1] as RequestInit, payload: JSON.parse((c[1] as RequestInit).body as string) as ExportBody }));
		const exported = exports_.find(
			(e) => e.payload.resourceSpans[0]?.scopeSpans[0]?.spans[0]?.traceId === traceId
		);
		expect(exported).toBeDefined();
		expect(exported?.init.method).toBe('POST');
		expect(exported?.init.keepalive).toBe(true);
		const span = exported?.payload.resourceSpans[0]?.scopeSpans[0]?.spans[0];
		expect(span?.name).toBe('browser.pageview');
		expect(span?.spanId).toBe(spanId);
		expect(span?.attributes.some((a) => a.key === 'url.path')).toBe(true);
	});
});

describe('createVitalsCollector', () => {
	it('accumulates each vital from its performance entry type', () => {
		const collector = createVitalsCollector();
		collector.record([
			entry({ entryType: 'paint', name: 'first-contentful-paint', startTime: 120 }),
			entry({ entryType: 'largest-contentful-paint', startTime: 300 }),
			entry({ entryType: 'largest-contentful-paint', startTime: 450 }),
			entry({ entryType: 'layout-shift', hadRecentInput: false, value: 0.05 }),
			entry({ entryType: 'layout-shift', hadRecentInput: true, value: 0.9 }),
			entry({ entryType: 'event', duration: 40 }),
			entry({ entryType: 'event', duration: 120 }),
			entry({ entryType: 'navigation', responseStart: 80 })
		]);

		expect(collector.snapshot()).toEqual({ lcp: 450, inp: 120, cls: 0.05, fcp: 120, ttfb: 80 });
	});
});

describe('buildPageviewExport', () => {
	it('emits the OTLP/JSON shape with measured vitals only', () => {
		const out = buildPageviewExport({
			traceId: 'a'.repeat(32),
			spanId: 'b'.repeat(16),
			startNanos: 1e9,
			endNanos: 2e9,
			vitals: { lcp: 100, inp: 0, cls: 0.1, fcp: 50, ttfb: 20 },
			path: '/vms'
		});

		const span = out.resourceSpans[0]?.scopeSpans[0]?.spans[0];
		expect(out.resourceSpans[0]?.resource.attributes).toEqual([{ key: 'service.name', value: { stringValue: 'pvmss-web' } }]);
		expect(span?.name).toBe('browser.pageview');
		expect(span?.kind).toBe(1);
		expect(span?.startTimeUnixNano).toBe('1000000000');
		expect(span?.endTimeUnixNano).toBe('2000000000');
		const attrs = new Map(span?.attributes.map((a) => [a.key, a.value]));
		expect(attrs.get('url.path')).toEqual({ stringValue: '/vms' });
		expect(attrs.get('web_vitals.lcp')).toEqual({ doubleValue: 100 });
		expect(attrs.get('web_vitals.cls')).toEqual({ doubleValue: 0.1 });
		expect(attrs.has('web_vitals.inp')).toBe(false);
	});
});
