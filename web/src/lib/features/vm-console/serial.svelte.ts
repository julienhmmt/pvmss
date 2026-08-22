import { getContext, setContext } from 'svelte';
import { Terminal } from '@xterm/xterm';
import { FitAddon } from '@xterm/addon-fit';
import { ApiRequestError } from '$lib/shared/api/client';
import { m } from '$lib/paraglide/messages.js';
import { buildSerialWebSocketURL, consoleTicketErrorMessage, fetchSerialTicket } from './console';

/** Connection states the serial console UI reflects. Mirrors ConsoleState. */
export type SerialState = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error';

/**
 * SerialConsoleStore — the Svelte 5 runes state for the xterm.js serial
 * terminal. One instance per console route (constitution VII: no module
 * singletons). Owns the xterm.js Terminal + FitAddon lifecycle and the raw
 * WebSocket byte tunnel to the backend's serial relay.
 *
 * The store is deliberately thin — it wraps xterm.js's API in $state fields the
 * template can bind to, and handles the Proxmox serial framing:
 *   - outgoing keystrokes: encoded as "0:len:data"
 *   - incoming output: parsed from "0:len:data"
 *   - resize: sent as "1:cols:rows:"
 *   - keepalive: responds to type "2" with "2"
 * PVMSS's server is a dumb byte pipe; the framing lives here in the browser.
 */
export class SerialConsoleStore {
	readonly cluster: string;
	readonly vmid: number;

	state = $state<SerialState>('idle');
	error = $state<string | null>(null);

	#term: Terminal | null = null;
	#fit: FitAddon | null = null;
	#ws: WebSocket | null = null;
	#container: HTMLElement | null = null;
	#keepaliveTimer: ReturnType<typeof setInterval> | null = null;

	constructor(cluster: string, vmid: number) {
		this.cluster = cluster;
		this.vmid = vmid;
	}

	/** Attaches the xterm.js Terminal to a target element and connects. */
	async connect(container: HTMLElement): Promise<void> {
		if (this.#term !== null || this.state === 'connecting' || this.state === 'connected') return;

		this.#container = container;
		this.state = 'connecting';
		this.error = null;

		const term = new Terminal({
			cursorBlink: true,
			fontFamily: 'monospace',
			fontSize: 14,
			theme: { background: '#000000', foreground: '#e0e0e0' }
		});
		const fit = new FitAddon();
		term.loadAddon(fit);
		term.open(container);
		try {
			fit.fit();
		} catch {
			// fit can throw if the container has no dimensions yet; non-fatal
		}

		this.#term = term;
		this.#fit = fit;

		term.onData((data: string) => this.#sendInput(data));
		term.onResize(({ cols, rows }) => this.#sendResize(cols, rows));

		let token: string;
		try {
			token = await fetchSerialTicket(this.cluster, this.vmid);
		} catch (err) {
			this.state = 'error';
			this.error = consoleTicketErrorMessage(err, () => m['vms.console.errorTicket']());
			return;
		}

		const url = buildSerialWebSocketURL(this.cluster, this.vmid, token);
		const ws = new WebSocket(url);
		ws.binaryType = 'arraybuffer';
		this.#ws = ws;

		ws.addEventListener('open', () => this.#onOpen());
		ws.addEventListener('message', (e) => this.#onMessage(e));
		ws.addEventListener('close', () => this.#onClose());
		ws.addEventListener('error', () => this.#onError());
	}

	/** Disconnects and releases the xterm.js Terminal + WebSocket. Safe when idle. */
	disconnect(): void {
		this.#stopKeepalive();
		if (this.#ws !== null) {
			this.#ws.close();
			this.#ws = null;
		}
		if (this.#term !== null) {
			this.#term.dispose();
			this.#term = null;
		}
		this.#fit = null;
		this.state = 'disconnected';
	}

	/** Reconnects after a disconnect. Reuses the stored container. */
	async reconnect(): Promise<void> {
		this.disconnect();
		if (this.#container !== null) {
			await this.connect(this.#container);
		}
	}

	/** Refits the terminal to its container. Called on mode switch / resize. */
	fit(): void {
		if (this.#fit !== null) {
			try {
				this.#fit.fit();
			} catch {
				// non-fatal
			}
		}
	}

	#onOpen(): void {
		this.state = 'connected';
		this.error = null;
		// Send initial resize so the remote knows our dimensions.
		if (this.#term !== null) {
			this.#sendResize(this.#term.cols, this.#term.rows);
		}
		// Proxmox expects periodic keepalive pings; send one every 30s.
		this.#startKeepalive();
	}

	#onMessage(event: MessageEvent): void {
		const data = event.data;
		if (typeof data === 'string') {
			this.#handleFrame(data);
		} else if (data instanceof ArrayBuffer) {
			this.#handleFrame(new TextDecoder().decode(data));
		}
	}

	/**
	 * Parses a Proxmox serial frame and writes output to the terminal.
	 * Framing: "type:payload" where type is 0 (data), 1 (resize), 2 (keepalive).
	 * Data frames: "0:len:chars" — write chars to the terminal.
	 * Keepalive frames: "2" — respond with "2".
	 */
	#handleFrame(frame: string): void {
		const sep = frame.indexOf(':');
		if (sep < 0) return;

		const type = frame.slice(0, sep);
		const rest = frame.slice(sep + 1);

		if (type === '0') {
			// "0:len:data" — extract data after the second colon.
			const lenSep = rest.indexOf(':');
			if (lenSep < 0) return;
			const payload = rest.slice(lenSep + 1);
			this.#term?.write(payload);
		} else if (type === '2') {
			// Keepalive ping from server — respond with "2".
			this.#ws?.send('2');
		}
	}

	#onClose(): void {
		this.#stopKeepalive();
		this.state = 'disconnected';
		this.#ws = null;
	}

	#onError(): void {
		this.state = 'error';
		this.error = m['vms.console.connectionFailed']();
	}

	/** Encodes keystrokes as "0:len:data" and sends to the server. */
	#sendInput(data: string): void {
		if (this.#ws === null || this.#ws.readyState !== WebSocket.OPEN) return;
		const encoded = new TextEncoder().encode(data);
		const frame = `0:${encoded.length}:${data}`;
		this.#ws.send(frame);
	}

	/** Sends a resize frame "1:cols:rows:" to the server. */
	#sendResize(cols: number, rows: number): void {
		if (this.#ws === null || this.#ws.readyState !== WebSocket.OPEN) return;
		this.#ws.send(`1:${cols}:${rows}:`);
	}

	#startKeepalive(): void {
		this.#stopKeepalive();
		this.#keepaliveTimer = setInterval(() => {
			if (this.#ws !== null && this.#ws.readyState === WebSocket.OPEN) {
				this.#ws.send('2');
			}
		}, 30_000);
	}

	#stopKeepalive(): void {
		if (this.#keepaliveTimer !== null) {
			clearInterval(this.#keepaliveTimer);
			this.#keepaliveTimer = null;
		}
	}
}

const SERIAL_CONTEXT_KEY = Symbol('vm-serial-console');

/** Called once by the console route that owns this state (constitution VII). */
export function setSerialConsoleContext(cluster: string, vmid: number): SerialConsoleStore {
	const store = new SerialConsoleStore(cluster, vmid);
	setContext(SERIAL_CONTEXT_KEY, store);
	return store;
}

export function getSerialConsoleContext(): SerialConsoleStore {
	return getContext<SerialConsoleStore>(SERIAL_CONTEXT_KEY);
}

export { ApiRequestError };
