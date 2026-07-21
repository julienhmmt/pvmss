<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { page } from '$app/stores';
	import { t } from 'svelte-i18n';
	import { buildWebSocketURL, getVNCTicket } from '$lib/api/console';

	const vmid = $derived(parseInt($page.params.id ?? '', 10));
	const vmName = $derived(
		$page.url.searchParams.get('name') ??
			(Number.isNaN(vmid) || vmid <= 0 ? $t('common.vm') : `${$t('common.vm')} ${vmid}`)
	);

	type ConnectionStatus = 'idle' | 'connecting' | 'connected' | 'disconnected' | 'error';

	let status = $state<ConnectionStatus>('idle');
	let statusMessageKey = $state('');
	let statusMessageValues = $state<Record<string, unknown>>({});
	let statusMessageSuffix = $state('');
	const statusMessage = $derived(
		statusMessageKey
			? `${$t(statusMessageKey, { values: statusMessageValues })}${statusMessageSuffix}`
			: ''
	);
	let mounted = $state(false);

	let rfb: unknown = null;
	let container: HTMLDivElement | undefined = $state();
	let scaleViewport = $state(true);

	function setStatusMessage(key: string, values?: Record<string, unknown>, suffix?: string) {
		statusMessageKey = key;
		statusMessageValues = values ?? {};
		statusMessageSuffix = suffix ?? '';
	}

	async function connect() {
		if (!mounted || !container || !vmid) return;
		// Guard against concurrent/duplicate connects (e.g. a remount firing
		// onMount again, or a Reconnect click while still connecting). Each VNC
		// ticket spawns a distinct Proxmox VNC session; opening two for the same
		// VM makes them collide and both drop with a 1006 abnormal closure.
		if (status === 'connecting' || status === 'connected') return;
		// Tear down any leftover session before starting a fresh one.
		if (rfb) {
			(rfb as { disconnect(): void }).disconnect();
			rfb = null;
		}

		status = 'connecting';
		setStatusMessage('vm.console.requestingTicket');

		let ticket: string;
		let port: number;
		let node: string;
		let consoleToken: string | undefined;

		try {
			const res = await getVNCTicket(vmid);
			ticket = res.ticket;
			port = res.port;
			node = res.node;
			consoleToken = res.consoleToken;
		} catch (e) {
			status = 'error';
			setStatusMessage('vm.console.failedTicket', {}, ': ' + (e as Error).message);
			return;
		}

		if (!mounted) return;
		setStatusMessage('vm.console.connecting');

		try {
			// noVNC lives in static/ — loaded at runtime, never bundled.
			const rfbUrl = '/noVNC-1.6.0/core/rfb.js';
			// eslint-disable-next-line @typescript-eslint/ban-ts-comment
			// @ts-ignore
			const { default: RFB } = await import(/* @vite-ignore */ rfbUrl);

			if (!RFB) {
				throw new Error('noVNC RFB module failed to load');
			}

			if (!mounted) return;

			const wsUrl = buildWebSocketURL(vmid, ticket, port, node, consoleToken);

			rfb = new (RFB as { new (...args: unknown[]): unknown })(container, wsUrl, {
				credentials: { username: '', password: ticket }
			});

			const r = rfb as {
				scaleViewport: boolean;
				resizeSession: boolean;
				addEventListener(event: string, handler: () => void): void;
				disconnect(): void;
			};

			r.scaleViewport = scaleViewport;
			r.resizeSession = false;

			r.addEventListener('connect', () => {
				status = 'connected';
				setStatusMessage('vm.console.connected');
			});

			r.addEventListener('disconnect', () => {
				status = 'disconnected';
				setStatusMessage('vm.console.disconnected');
				rfb = null;
			});

			r.addEventListener('securityfailure', () => {
				status = 'error';
				setStatusMessage('vm.console.authFailed');
				rfb = null;
			});
		} catch (e) {
			status = 'error';
			setStatusMessage('vm.console.failedInit', {}, ': ' + (e as Error).message);
		}
	}

	function disconnect() {
		if (rfb) {
			(rfb as { disconnect(): void }).disconnect();
		}
	}

	function toggleScale() {
		scaleViewport = !scaleViewport;
		if (rfb) {
			(rfb as { scaleViewport: boolean }).scaleViewport = scaleViewport;
		}
	}

	function sendCtrlAltDel() {
		if (rfb) {
			(rfb as { sendCtrlAltDel(): void }).sendCtrlAltDel();
		}
	}

	onMount(() => {
		mounted = true;
		if (vmid > 0) {
			connect();
		} else {
			status = 'error';
			setStatusMessage('vm.console.noVmid');
		}
	});

	onDestroy(() => {
		mounted = false;
		disconnect();
	});
</script>

<svelte:head>
	<title>PVMSS — {$t('vm.console.title')} — {vmName}</title>
</svelte:head>

<div class="console-root">
	<!-- Toolbar -->
	<div class="console-toolbar">
		<span class="console-vm-name">{vmName}</span>

		<div class="console-toolbar-center">
			<span
				class="console-status console-status--{status === 'connected'
					? 'ok'
					: status === 'error' || status === 'disconnected'
						? 'err'
						: 'pending'}"
			>
				{statusMessage || status}
			</span>
		</div>

		<div class="console-toolbar-actions">
			{#if status === 'connected'}
				<button class="console-btn" onclick={sendCtrlAltDel} title={$t('common.sendCtrlAltDel')}>
					{$t('common.ctrlAltDel')}
				</button>
				<button
					class="console-btn {scaleViewport ? 'console-btn--active' : ''}"
					onclick={toggleScale}
					title={$t('common.toggleScaling')}
				>
					{$t('common.scale')}
				</button>
			{/if}
			{#if status === 'disconnected' || status === 'error'}
				<button class="console-btn" onclick={connect}>{$t('common.reconnect')}</button>
			{/if}
			{#if status === 'connected'}
				<button class="console-btn console-btn--danger" onclick={disconnect}>{$t('common.disconnect')}</button>
			{/if}
		</div>
	</div>

	<!-- Canvas container -->
	<div class="console-canvas-wrap">
		{#if status === 'idle' || status === 'connecting'}
			<div class="console-overlay">
				<div class="console-spinner"></div>
				<p class="console-overlay-msg">{statusMessage}</p>
			</div>
		{:else if status === 'error' || status === 'disconnected'}
			<div class="console-overlay">
				<p class="console-overlay-msg console-overlay-msg--error">{statusMessage}</p>
				<button class="console-btn mt-3" onclick={connect}>{$t('common.reconnect')}</button>
			</div>
		{/if}
		<div bind:this={container} class="console-canvas" class:console-canvas--hidden={status !== 'connected'}></div>
	</div>
</div>

<style>
	.console-root {
		position: fixed;
		inset: 0;
		z-index: 9999;
		display: flex;
		flex-direction: column;
		background: #1a1a1a;
		color: #e0e0e0;
		font-family: ui-sans-serif, system-ui, sans-serif;
	}

	.console-toolbar {
		display: flex;
		align-items: center;
		justify-content: space-between;
		padding: 0 0.75rem;
		height: 2.5rem;
		background: #111;
		border-bottom: 1px solid #333;
		flex-shrink: 0;
		gap: 0.5rem;
	}

	.console-vm-name {
		font-size: 0.8rem;
		font-weight: 600;
		color: #ccc;
		white-space: nowrap;
		overflow: hidden;
		text-overflow: ellipsis;
		max-width: 14rem;
	}

	.console-toolbar-center {
		flex: 1;
		display: flex;
		justify-content: center;
	}

	.console-toolbar-actions {
		display: flex;
		gap: 0.375rem;
	}

	.console-status {
		font-size: 0.7rem;
		padding: 0.15rem 0.5rem;
		border-radius: 9999px;
		font-weight: 500;
	}
	.console-status--ok { background: #14532d; color: #86efac; }
	.console-status--err { background: #450a0a; color: #fca5a5; }
	.console-status--pending { background: #1c1917; color: #a3a3a3; }

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
	.console-btn:hover { background: #3a3a3a; }
	.console-btn--active { background: #1d4ed8; border-color: #2563eb; color: #fff; }
	.console-btn--danger { border-color: #7f1d1d; color: #fca5a5; }
	.console-btn--danger:hover { background: #450a0a; }

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
	.console-overlay-msg--error { color: #fca5a5; }

	.console-spinner {
		width: 2rem;
		height: 2rem;
		border: 3px solid #444;
		border-top-color: #60a5fa;
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}
	@keyframes spin { to { transform: rotate(360deg); } }

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
</style>
