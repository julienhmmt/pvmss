import { afterEach, beforeEach, describe, expect, it, vi } from 'vitest';
import { SerialConsoleStore } from './serial.svelte';
import { fetchSerialTicket } from './console';

const fakes = vi.hoisted(() => {
	const sockets: FakeWebSocket[] = [];
	const terminals: FakeTerminal[] = [];

	class FakeWebSocket extends EventTarget {
		static readonly OPEN = 1;
		readyState = 0;
		binaryType = 'blob';
		close = vi.fn();
		send = vi.fn();

		constructor(readonly url: string) {
			super();
			sockets.push(this);
		}

		/** Simulates the server accepting the tunnel. */
		open(): void {
			this.readyState = FakeWebSocket.OPEN;
			this.dispatchEvent(new Event('open'));
		}

		/** Simulates the tunnel closing from the server side. */
		closeFromServer(): void {
			this.readyState = 3;
			this.dispatchEvent(new Event('close'));
		}
	}

	class FakeTerminal {
		cols = 80;
		rows = 24;
		dispose = vi.fn();
		write = vi.fn();

		constructor() {
			terminals.push(this);
		}

		loadAddon(): void {}
		open(): void {}
		onData(): void {}
		onResize(): void {}
	}

	class FakeFitAddon {
		fit(): void {}
	}

	return { sockets, terminals, FakeWebSocket, FakeTerminal, FakeFitAddon };
});

vi.mock('@xterm/xterm', () => ({ Terminal: fakes.FakeTerminal }));
vi.mock('@xterm/addon-fit', () => ({ FitAddon: fakes.FakeFitAddon }));

vi.mock('./console', async (importOriginal) => {
	const actual = await importOriginal<typeof import('./console')>();
	return { ...actual, fetchSerialTicket: vi.fn() };
});

type FakeWebSocket = InstanceType<typeof fakes.FakeWebSocket>;

const ticketMock = vi.mocked(fetchSerialTicket);

/** Returns a handle that resolves the next ticket fetch, for out-of-order tests. */
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
	fakes.sockets.length = 0;
	fakes.terminals.length = 0;
	ticketMock.mockReset();
	ticketMock.mockResolvedValue('token-a');
	vi.stubGlobal('WebSocket', fakes.FakeWebSocket);
});

afterEach(() => vi.unstubAllGlobals());

describe('SerialConsoleStore.connect', () => {
	it('opens one WebSocket once the ticket resolves', async () => {
		const store = new SerialConsoleStore('default', 100);

		await store.connect(document.createElement('div'));

		expect(fakes.sockets).toHaveLength(1);
		expect(store.state).toBe('connecting');
		store.disconnect();
	});

	it('opens no WebSocket when disconnect runs before the ticket resolves', async () => {
		const store = new SerialConsoleStore('default', 100);
		const ticket = deferredTicket();

		const pending = store.connect(document.createElement('div'));
		store.disconnect();
		ticket.resolve('token-late');
		await pending;

		expect(fakes.sockets).toHaveLength(0);
		expect(store.state).toBe('disconnected');
	});
});

describe('SerialConsoleStore listeners', () => {
	it('ignores a late close from a replaced socket', async () => {
		const store = new SerialConsoleStore('default', 100);
		const container = document.createElement('div');

		await store.connect(container);
		await store.reconnect();

		const [first, second] = fakes.sockets as [FakeWebSocket, FakeWebSocket];
		first.closeFromServer();

		store.disconnect();
		expect(second.close).toHaveBeenCalledTimes(1);
	});

	it('does not report a no-serial error after a reconnect that never opened', async () => {
		const store = new SerialConsoleStore('default', 100);
		const container = document.createElement('div');

		await store.connect(container);
		fakes.sockets[0]?.open();
		store.disconnect();
		await store.connect(container);

		fakes.sockets[1]?.closeFromServer();

		expect(store.state).toBe('disconnected');
		expect(store.error).toBeNull();
		store.disconnect();
	});
});
