import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { ConsoleStore } from './console.svelte';
import { fetchConsoleTicket } from './console';

const fakes = vi.hoisted(() => {
	const instances: FakeRFB[] = [];

	class FakeRFB extends EventTarget {
		scaleViewport = false;
		disconnect = vi.fn();
		sendCtrlAltDel = vi.fn();
		clipboardPasteFrom = vi.fn();

		constructor(
			readonly target: HTMLElement,
			readonly url: string,
			readonly options: { shared?: boolean }
		) {
			super();
			instances.push(this);
		}
	}

	return { instances, FakeRFB };
});

vi.mock('@novnc/novnc', () => ({ default: fakes.FakeRFB }));

vi.mock('./console', async (importOriginal) => {
	const actual = await importOriginal<typeof import('./console')>();
	return { ...actual, fetchConsoleTicket: vi.fn() };
});

type FakeRFB = InstanceType<typeof fakes.FakeRFB>;

const ticketMock = vi.mocked(fetchConsoleTicket);

/** Returns a promise plus the function that resolves it, for out-of-order fetches. */
function deferredTicket(): { resolve: (token: string) => void } {
	let resolve: (token: string) => void = () => {};
	ticketMock.mockImplementationOnce(
		() =>
			new Promise<string>((r) => {
				resolve = r;
			})
	);
	return { resolve: (token: string) => resolve(token) };
}

beforeEach(() => {
	fakes.instances.length = 0;
	ticketMock.mockReset();
	ticketMock.mockResolvedValue('token-a');
});

afterEach(() => vi.unstubAllGlobals());

describe('ConsoleStore.connect', () => {
	it('creates one RFB for the container once the ticket resolves', async () => {
		const store = new ConsoleStore('default', 100);
		const container = document.createElement('div');

		await store.connect(container);

		expect(fakes.instances).toHaveLength(1);
		expect(fakes.instances[0]?.target).toBe(container);
		expect(store.state).toBe('connecting');
	});

	it('creates no RFB when disconnect runs before the ticket resolves', async () => {
		const store = new ConsoleStore('default', 100);
		const ticket = deferredTicket();

		const pending = store.connect(document.createElement('div'));
		store.disconnect();
		ticket.resolve('token-late');
		await pending;

		expect(fakes.instances).toHaveLength(0);
		expect(store.state).toBe('disconnected');
	});
});

describe('ConsoleStore listeners', () => {
	it('ignores a late disconnect from a replaced RFB', async () => {
		const store = new ConsoleStore('default', 100);
		const container = document.createElement('div');

		await store.connect(container);
		store.disconnect();
		await store.connect(container);

		const [first, second] = fakes.instances as [FakeRFB, FakeRFB];
		first.dispatchEvent(new CustomEvent('disconnect', { detail: { clean: false } }));

		expect(store.state).toBe('connecting');
		expect(store.error).toBeNull();

		store.disconnect();
		expect(second.disconnect).toHaveBeenCalledTimes(1);
	});
});
