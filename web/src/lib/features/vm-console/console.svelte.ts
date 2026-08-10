import { getContext, setContext } from 'svelte';
import RFB from '@novnc/novnc';
import { ApiRequestError } from '$lib/shared/api/client';
import { buildConsoleWebSocketURL, consoleTicketErrorMessage, fetchConsoleTicket } from './console';

/** Connection states the console UI reflects. */
export type ConsoleState = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error';

/** Clipboard direction for the paste-into-VM action. */
export type ClipboardState = {
	/** Text received from the remote VM via ServerCutText. */
	fromVM: string;
	/** Whether the remote server has clipboard capabilities. */
	serverHasClipboard: boolean;
};

/**
 * ConsoleStore — the Svelte 5 runes state for the VNC console. One instance
 * per console route (constitution VII: no module singletons). Owns the RFB
 * instance lifecycle: connect, disconnect, reconnect, scale, Ctrl+Alt+Del,
 * clipboard both ways.
 *
 * The store is deliberately thin — it wraps RFB's event-driven API in
 * $state fields the template can bind to, and nothing more. All the hard
 * parts (handshake, framebuffer decoding, input encoding) are noVNC's job.
 */
export class ConsoleStore {
	readonly cluster: string;
	readonly vmid: number;

	state = $state<ConsoleState>('idle');
	error = $state<string | null>(null);
	scaleViewport = $state(true);
	clipboard = $state<ClipboardState>({ fromVM: '', serverHasClipboard: false });

	#rfb: RFB | null = null;
	#container: HTMLElement | null = null;

	constructor(cluster: string, vmid: number) {
		this.cluster = cluster;
		this.vmid = vmid;
	}

	/** Attaches the RFB canvas to a target element and connects. */
	async connect(container: HTMLElement): Promise<void> {
		if (this.#rfb !== null || this.state === 'connecting' || this.state === 'connected') return;

		this.#container = container;
		this.state = 'connecting';
		this.error = null;

		let token: string;
		try {
			token = await fetchConsoleTicket(this.cluster, this.vmid);
		} catch (err) {
			this.state = 'error';
			this.error = consoleTicketErrorMessage(err, 'failed to obtain console ticket');
			return;
		}

		const url = buildConsoleWebSocketURL(this.cluster, this.vmid, token);
		try {
			this.#rfb = new RFB(container, url, { shared: true });
		} catch (err) {
			this.state = 'error';
			this.error = err instanceof Error ? err.message : 'failed to create RFB connection';
			return;
		}

		this.#rfb.scaleViewport = this.scaleViewport;
		this.#rfb.addEventListener('connect', () => this.#onConnect());
		this.#rfb.addEventListener('disconnect', (e) => this.#onDisconnect(e));
		this.#rfb.addEventListener('securityfailure', (e) => this.#onSecurityFailure(e));
		this.#rfb.addEventListener('clipboard', (e) => this.#onClipboard(e));
		this.#rfb.addEventListener('capabilities', (e) => this.#onCapabilities(e));
	}

	/** Disconnects and releases the RFB instance. Safe to call when idle. */
	disconnect(): void {
		if (this.#rfb !== null) {
			this.#rfb.disconnect();
			this.#rfb = null;
		}
		this.state = 'disconnected';
	}

	/** Reconnects after a disconnect. Reuses the stored container. */
	async reconnect(): Promise<void> {
		this.disconnect();
		if (this.#container !== null) {
			await this.connect(this.#container);
		}
	}

	/** Toggles scale-to-fit and applies it to the live RFB instance. */
	toggleScale(): void {
		this.scaleViewport = !this.scaleViewport;
		if (this.#rfb !== null) {
			this.#rfb.scaleViewport = this.scaleViewport;
		}
	}

	/** Sends Ctrl+Alt+Del to the remote VM. No-op if not connected. */
	sendCtrlAltDel(): void {
		if (this.#rfb !== null && this.state === 'connected') {
			this.#rfb.sendCtrlAltDel();
		}
	}

	/** Pastes text from the local clipboard into the remote VM. */
	pasteToVM(text: string): void {
		if (this.#rfb !== null && this.state === 'connected' && text !== '') {
			this.#rfb.clipboardPasteFrom(text);
		}
	}

	/** Reads the local clipboard and pastes it into the remote VM. */
	async pasteFromLocalClipboard(): Promise<void> {
		if (navigator.clipboard === undefined) {
			this.error = 'Clipboard API is not available in this browser';
			return;
		}
		try {
			const text = await navigator.clipboard.readText();
			this.pasteToVM(text);
		} catch {
			this.error = 'Clipboard permission denied — cannot read from your local clipboard';
		}
	}

	/** Copies the latest clipboard text received from the VM to the local clipboard. */
	async copyFromVMToLocal(): Promise<void> {
		if (this.clipboard.fromVM === '') return;
		if (navigator.clipboard === undefined) {
			this.error = 'Clipboard API is not available in this browser';
			return;
		}
		try {
			await navigator.clipboard.writeText(this.clipboard.fromVM);
		} catch {
			this.error = 'Clipboard permission denied — cannot write to your local clipboard';
		}
	}

	#onConnect(): void {
		this.state = 'connected';
		this.error = null;
	}

	#onDisconnect(e: Event): void {
		const detail = (e as CustomEvent).detail;
		this.state = 'disconnected';
		if (detail && typeof detail === 'object' && 'clean' in detail && !detail.clean) {
			this.error = 'Connection lost unexpectedly';
		}
		this.#rfb = null;
	}

	#onSecurityFailure(e: Event): void {
		const detail = (e as CustomEvent).detail;
		this.state = 'error';
		this.error = detail && typeof detail === 'object' && 'reason' in detail
			? `Security failure: ${detail.reason}`
			: 'Security failure';
	}

	#onClipboard(e: Event): void {
		const detail = (e as CustomEvent).detail;
		if (detail && typeof detail === 'object' && 'text' in detail && typeof detail.text === 'string') {
			this.clipboard = { ...this.clipboard, fromVM: detail.text };
		}
	}

	#onCapabilities(e: Event): void {
		const detail = (e as CustomEvent).detail;
		if (detail && typeof detail === 'object' && 'capabilities' in detail) {
			const caps = detail.capabilities as Record<string, boolean> | undefined;
			this.clipboard = { ...this.clipboard, serverHasClipboard: caps?.clipboard ?? false };
		}
	}
}

const CONSOLE_CONTEXT_KEY = Symbol('vm-console');

/** Called once by the console route that owns this state (constitution VII). */
export function setConsoleContext(cluster: string, vmid: number): ConsoleStore {
	const store = new ConsoleStore(cluster, vmid);
	setContext(CONSOLE_CONTEXT_KEY, store);
	return store;
}

export function getConsoleContext(): ConsoleStore {
	return getContext<ConsoleStore>(CONSOLE_CONTEXT_KEY);
}

export { ApiRequestError };
