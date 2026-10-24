import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { MetricsStore } from './metrics.svelte';

type StubEventSource = EventSource & {
	url: string;
	closed: boolean;
};

class FakeEventSource {
	url: string;
	closed = false;
	onopen: ((this: EventSource, ev: Event) => void) | null = null;
	onmessage: ((this: EventSource, ev: MessageEvent) => void) | null = null;
	onerror: ((this: EventSource, ev: Event) => void) | null = null;

	constructor(url: string) {
		this.url = url;
	}

	close() {
		this.closed = true;
	}
}

let lastSource: StubEventSource | null = null;

function makeEventSourceConstructor() {
	return function (this: unknown, url: string) {
		lastSource = new FakeEventSource(url) as unknown as StubEventSource;
		return lastSource;
	} as unknown as typeof EventSource;
}

describe('MetricsStore', () => {
	beforeEach(() => {
		lastSource = null;
		vi.stubGlobal('EventSource', makeEventSourceConstructor());
	});

	afterEach(() => {
		vi.unstubAllGlobals();
		vi.useRealTimers();
		lastSource = null;
	});

	it('does not connect when the VM is not running', () => {
		const store = new MetricsStore('default', 100, () => false);
		store.connect();

		expect(lastSource).toBeNull();
		expect(store.streamState).toBe('idle');
	});

	it('opens an EventSource to the metrics stream when running', () => {
		const store = new MetricsStore('default', 100, () => true);
		store.connect();

		expect(store.streamState).toBe('connecting');
		expect(lastSource).not.toBeNull();
		expect(lastSource!.url).toBe('/api/v1/vms/default/100/metrics/stream');

		lastSource!.onopen?.(new Event('open'));
		expect(store.streamState).toBe('connected');
	});

	it('merges live ticks into the samples list', () => {
		const store = new MetricsStore('default', 100, () => true);
		store.connect();

		lastSource!.onmessage?.(
			new MessageEvent('message', {
				data: JSON.stringify({
					timestamp: '2026-01-01T00:00:00Z',
					cpuPercent: 42,
					memoryUsedBytes: 1,
					memoryMaxBytes: 2,
					diskReadBytesPerSec: 3,
					diskWriteBytesPerSec: 4,
					netInBytesPerSec: 5,
					netOutBytesPerSec: 6
				})
			})
		);

		expect(store.samples.length).toBe(1);
		expect(store.samples[0]?.cpuPercent).toBe(42);
	});

	it('closes the EventSource on disconnect', () => {
		const store = new MetricsStore('default', 100, () => true);
		store.connect();
		store.disconnect();

		expect(lastSource?.closed).toBe(true);
		expect(store.streamState).toBe('idle');
	});

	it('does not open a second EventSource if already connected', () => {
		const store = new MetricsStore('default', 100, () => true);
		store.connect();
		const first = lastSource;
		store.connect();

		expect(lastSource).toBe(first);
	});

	it('triggers reconnect state on EventSource error', () => {
		vi.useFakeTimers();
		const store = new MetricsStore('default', 100, () => true);
		store.connect();

		lastSource!.onerror?.(new Event('error'));

		expect(store.streamState).toBe('reconnecting');
	});
});
