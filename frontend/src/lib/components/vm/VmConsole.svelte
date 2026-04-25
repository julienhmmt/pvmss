<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { getVNCTicket, buildWebSocketURL } from '$lib/api/console';

	interface RFB {
		scaleViewport: boolean;
		resizeSession: boolean;
		connect(url: string, credentials?: { username?: string; password?: string }): void;
		disconnect(): void;
		sendKey(key: number, code?: string): void;
		sendCredentials(credentials: { username?: string; password?: string }): void;
		focus(): void;
		blur(): void;
		clipboardPasteFrom(text: string): void;
		clipboardPasteTo(callback: (text: string) => void): void;
		sendCtrlAltDel(): void;
		sendText(text: string): void;
		resize(width: number, height: number): void;
		getDisplay(): { width: number; height: number };
		addEventListener(event: string, callback: (...args: unknown[]) => void): void;
		removeEventListener(event: string, callback: (...args: unknown[]) => void): void;
	}

	interface Props {
		vmid: number;
	}

	let { vmid }: Props = $props();

	type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error';

	let status = $state<ConnectionStatus>('idle');
	let statusMessage = $state('');
	let rfb: RFB | null = null;
	let container: HTMLDivElement | undefined = $state();
	let scaleViewport = $state(true);
	let mounted = $state(true);
	let connectHandler: ((...args: unknown[]) => void) | null = null;
	let disconnectHandler: ((...args: unknown[]) => void) | null = null;
	let securityFailureHandler: ((...args: unknown[]) => void) | null = null;
	let retryCount = $state(0);
	let retryTimeout: ReturnType<typeof setTimeout> | null = null;

	export async function connect() {
		if (!container || vmid <= 0) return;
		if (!mounted) return;

		status = 'connecting';
		statusMessage = 'Requesting VNC ticket…';

		let ticket: string;
		let port: number;
		let node: string;

		try {
			const res = await getVNCTicket(vmid);
			if (!mounted) return;

			ticket = res.ticket;
			port = res.port;
			node = res.node;
		} catch (e) {
			if (!mounted) return;
			status = 'error';
			statusMessage = `Failed to get VNC ticket: ${(e as Error).message}`;
			scheduleRetry();
			return;
		}

		statusMessage = 'Connecting to console…';

		try {
			const rfbUrl = '/noVNC-1.6.0/core/rfb.js';
			const { default: RFBModule } = await import(/* @vite-ignore */ rfbUrl);

			if (!RFBModule) throw new Error('noVNC RFB module failed to load');

			const wsUrl = buildWebSocketURL(vmid, ticket, port, node);

			const oldRfb = rfb;
			if (oldRfb) {
				if (connectHandler) {
					oldRfb.removeEventListener('connect', connectHandler);
				}
				if (disconnectHandler) {
					oldRfb.removeEventListener('disconnect', disconnectHandler);
				}
				if (securityFailureHandler) {
					oldRfb.removeEventListener('securityfailure', securityFailureHandler);
				}
				oldRfb.disconnect();
			}

			rfb = new RFBModule(container, wsUrl, {
				credentials: { username: '', password: ticket }
			});

			if (!mounted) return;

			const currentRfb = rfb;
			if (!currentRfb) return;

			currentRfb.scaleViewport = scaleViewport;
			currentRfb.resizeSession = false;

			connectHandler = () => {
				if (!mounted) return;
				status = 'connected';
				statusMessage = 'Connected';
				retryCount = 0;
			};
			disconnectHandler = (detail: unknown) => {
				if (rfb === currentRfb) rfb = null;
				if (!mounted) return;
				status = 'disconnected';
				const reason = detail && typeof detail === 'object' && 'reason' in detail
					? `: ${String((detail as { reason: string }).reason)}`
					: '';
				statusMessage = `Disconnected${reason}`;
			};
			securityFailureHandler = () => {
				if (rfb === currentRfb) rfb = null;
				if (!mounted) return;
				status = 'error';
				statusMessage = 'Authentication failed';
			};

			currentRfb.addEventListener('connect', connectHandler);
			currentRfb.addEventListener('disconnect', disconnectHandler);
			currentRfb.addEventListener('securityfailure', securityFailureHandler);
		} catch (e) {
			if (!mounted) return;
			const err = e as Error;
			if (err.name === 'TypeError' && (err.message.includes('fetch') || err.message.includes('Failed to fetch'))) {
				statusMessage = 'Failed to load console module. Check your network connection.';
			} else {
				statusMessage = `Failed to initialize console: ${err.message}`;
			}
			status = 'error';
			scheduleRetry();
		}
	}

	function scheduleRetry(): void {
		if (retryCount >= 3) return;
		const delay = Math.min(1000 * Math.pow(2, retryCount), 10000);
		retryCount++;
		statusMessage = `Retrying in ${delay / 1000}s... (${retryCount}/3)`;
		retryTimeout = setTimeout(() => {
			if (mounted) {
				connect();
			}
		}, delay);
	}

	export function disconnect() {
		mounted = false;
		if (retryTimeout) {
			clearTimeout(retryTimeout);
			retryTimeout = null;
		}
		if (rfb) {
			if (connectHandler) {
				rfb.removeEventListener('connect', connectHandler);
			}
			if (disconnectHandler) {
				rfb.removeEventListener('disconnect', disconnectHandler);
			}
			if (securityFailureHandler) {
				rfb.removeEventListener('securityfailure', securityFailureHandler);
			}
			rfb.disconnect();
			rfb = null;
		}
		connectHandler = null;
		disconnectHandler = null;
		securityFailureHandler = null;
		retryCount = 0;
	}

	export function toggleScale() {
		scaleViewport = !scaleViewport;
		if (rfb) {
			rfb.scaleViewport = scaleViewport;
		}
	}

	export function sendCtrlAltDel() {
		if (rfb) {
			rfb.sendCtrlAltDel();
		}
	}

	export { status, statusMessage, scaleViewport };

	onMount(() => {
		if (vmid > 0) connect();
		else {
			status = 'error';
			statusMessage = 'No VM ID provided';
		}
	});

	onDestroy(() => {
		disconnect();
	});
</script>

<div class="console-canvas-wrap">
	{#if status === 'idle' || status === 'connecting'}
		<div class="console-overlay">
			<div class="console-spinner"></div>
			<p class="console-overlay-msg">{statusMessage}</p>
		</div>
	{:else if status === 'error' || status === 'disconnected'}
		<div class="console-overlay">
			<p class="console-overlay-msg console-overlay-msg--error">{statusMessage}</p>
			<button class="console-btn mt-3" onclick={connect}>Reconnect</button>
		</div>
	{/if}
	<div
		bind:this={container}
		class="console-canvas"
		class:console-canvas--hidden={status !== 'connected'}
	></div>
</div>

<style>
	.console-canvas-wrap {
		flex: 1;
		position: relative;
		overflow: hidden;
		background: #000;
	}

	.console-overlay {
		position: absolute;
		inset: 0;
		display: flex;
		flex-direction: column;
		align-items: center;
		justify-content: center;
		background: rgba(0, 0, 0, 0.85);
		z-index: 10;
	}

	.console-overlay-msg {
		font-size: 0.875rem;
		color: #9ca3af;
		margin: 0.5rem 0 0;
	}

	.console-overlay-msg--error {
		color: #fca5a5;
	}

	.console-spinner {
		width: 2rem;
		height: 2rem;
		border: 3px solid #444;
		border-top-color: #60a5fa;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}

	.console-canvas {
		width: 100%;
		height: 100%;
	}

	.console-canvas--hidden {
		visibility: hidden;
	}

	:global(.console-canvas canvas) {
		display: block;
		width: 100% !important;
		height: 100% !important;
	}

	.console-btn {
		padding: 0.2rem 0.6rem;
		font-size: 0.7rem;
		background: #2a2a2a;
		color: #ccc;
		border: 1px solid #444;
		border-radius: 4px;
		cursor: pointer;
		white-space: nowrap;
		transition: background 0.1s;
	}

	.console-btn:hover {
		background: #3a3a3a;
	}
</style>
