<script lang="ts">
	import { t } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import { onMount, onDestroy } from 'svelte';
	import { GithubLogo, Info, CloudCheck, CloudSlash } from 'phosphor-svelte';

	interface HealthInfo {
		status: string;
		version: string;
	}

	interface ProxmoxHealth {
		connected: boolean;
		url: string;
	}

	let version = $state<string | null>(null);
	let proxmox = $state<ProxmoxHealth | null>(null);
	let mounted = $state(true);
	let loadError = $state(false);
	let year = $derived(new Date().getFullYear());
	let refreshInterval: ReturnType<typeof setInterval> | null = null;

	async function loadHealth(): Promise<void> {
		try {
			const [health, px] = await Promise.all([
				api.get<HealthInfo>('/api/v1/health'),
				api.get<ProxmoxHealth>('/api/v1/health/proxmox')
			]);
			if (!mounted) return;
			version = health.version;
			proxmox = px;
			loadError = false;
		} catch (err) {
			console.error('Failed to load health info:', err);
			if (!mounted) return;
			loadError = true;
		}
	}

	onMount(() => {
		loadHealth();
		refreshInterval = setInterval(loadHealth, 60000);
	});

	onDestroy(() => {
		mounted = false;
		if (refreshInterval) {
			clearInterval(refreshInterval);
		}
	});
</script>

<footer class="footer">
	<div class="footer-inner">
		<!-- Col 1: Brand -->
		<div class="footer-col">
			<div class="footer-brand-block">
				<span class="footer-logo">
					<span class="footer-logo-dot" aria-hidden="true"></span>
					PVMSS
				</span>
				<p class="footer-tagline">{$t('footer.tagline')}</p>
			</div>
			{#if version}
				<span class="footer-version">v{version}</span>
			{/if}
		</div>

		<!-- Col 2: Links -->
		<div class="footer-col">
			<p class="footer-col-title">{$t('footer.links')}</p>
			<nav class="footer-nav">
				<a class="footer-link" href="/">{$t('nav.home')}</a>
				<a class="footer-link" href="/home">{$t('nav.myVms')}</a>
				<a class="footer-link" href="/vm/create">{$t('nav.createVm')}</a>
				<a class="footer-link" href="https://j.hommet.net/pvmss" target="_blank" rel="noopener noreferrer">{$t('footer.documentation')}</a>
				<a
					class="footer-link"
					href="https://github.com/julienhmmt/pvmss"
					target="_blank"
					rel="noopener noreferrer"
				>
					<GithubLogo class="h-3.5 w-3.5" />
					{$t('footer.source')}
				</a>
			</nav>
		</div>

		<!-- Col 3: Status -->
		<div class="footer-col">
			<p class="footer-col-title">{$t('footer.status')}</p>
			{#if loadError}
				<p class="footer-status-loading">{$t('footer.statusUnavailable')}</p>
			{:else if proxmox !== null}
				<div class="footer-status-list">
					<div class="footer-status-row">
						{#if proxmox.connected}
							<CloudCheck class="h-3.5 w-3.5 footer-icon footer-icon--ok" />
							<span class="footer-status-text">{$t('footer.proxmoxConnected')}</span>
						{:else}
							<CloudSlash class="h-3.5 w-3.5 footer-icon footer-icon--err" />
							<span class="footer-status-text footer-status-text--err">
								{$t('footer.proxmoxDisconnected')}
							</span>
						{/if}
					</div>
					{#if proxmox.url}
						<div class="footer-status-row">
							<Info class="h-3.5 w-3.5 footer-icon" />
							<span class="footer-status-text footer-status-text--muted"
								>{new URL(proxmox.url).origin + new URL(proxmox.url).pathname.replace(/\/api2\/json$/, '')}</span
							>
						</div>
					{/if}
				</div>
			{:else}
				<p class="footer-status-loading">{$t('footer.loading')}</p>
			{/if}
			<button class="footer-link" onclick={() => window.scrollTo({ top: 0, behavior: 'smooth' })}>
				{$t('footer.backToTop')}
			</button>
		</div>
	</div>

	<!-- Bottom bar -->
	<div class="footer-bottom">
		<span>AGPL 3.0 - 2025-{year} PVMSS</span>
		<span class="footer-sep" aria-hidden="true">·</span>
	</div>
</footer>

<style>
	.footer {
		background: oklch(0.147 0.004 49.25);
		color: oklch(0.709 0.01 56.259);
		border-top: 1px solid oklch(1 0 0 / 8%);
		flex-shrink: 0;
		margin-top: auto;
	}

	.footer-inner {
		max-width: 1400px;
		margin: 0 auto;
		padding: 2rem 2rem 1.5rem;
		display: grid;
		grid-template-columns: 1.6fr 1fr 1fr;
		gap: 2.5rem;
	}

	/* ── Columns ────────────────────────────────────────────────────── */
	.footer-col {
		display: flex;
		flex-direction: column;
		gap: 0.75rem;
	}

	.footer-col-title {
		font-size: 0.6875rem;
		font-weight: 700;
		text-transform: uppercase;
		letter-spacing: 0.08em;
		color: oklch(0.985 0.001 106.423 / 30%);
		margin: 0;
	}

	/* ── Brand ──────────────────────────────────────────────────────── */
	.footer-brand-block {
		display: flex;
		flex-direction: column;
		gap: 0.4rem;
	}

	.footer-logo {
		display: flex;
		align-items: center;
		gap: 0.45rem;
		font-size: 1rem;
		font-weight: 700;
		color: oklch(0.985 0.001 106.423);
		letter-spacing: -0.01em;
	}

	.footer-logo-dot {
		display: inline-block;
		width: 8px;
		height: 8px;
		border-radius: 50%;
		background: oklch(0.68 0.19 45);
		flex-shrink: 0;
	}

	.footer-tagline {
		font-size: 0.8125rem;
		color: oklch(0.709 0.01 56.259);
		margin: 0;
		line-height: 1.45;
		max-width: 22rem;
	}

	.footer-version {
		font-size: 0.75rem;
		font-family: ui-monospace, monospace;
		color: oklch(0.709 0.01 56.259 / 60%);
	}

	/* ── Nav links ──────────────────────────────────────────────────── */
	.footer-nav {
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
	}

	.footer-link {
		display: flex;
		align-items: center;
		gap: 0.35rem;
		font-size: 0.8125rem;
		color: oklch(0.709 0.01 56.259);
		text-decoration: none;
		width: fit-content;
		transition: color 0.12s;
	}

	.footer-link:hover {
		color: oklch(0.985 0.001 106.423);
	}

	/* ── Status ─────────────────────────────────────────────────────── */
	.footer-status-list {
		display: flex;
		flex-direction: column;
		gap: 0.55rem;
	}

	.footer-status-row {
		display: flex;
		align-items: center;
		gap: 0.4rem;
	}

	:global(.footer-icon) {
		color: oklch(0.709 0.01 56.259 / 50%);
		flex-shrink: 0;
	}

	:global(.footer-icon--ok) {
		color: oklch(0.72 0.17 145);
	}

	:global(.footer-icon--err) {
		color: oklch(0.65 0.2 25);
	}

	.footer-status-text {
		font-size: 0.8125rem;
		color: oklch(0.709 0.01 56.259);
	}

	.footer-status-text--err {
		color: oklch(0.65 0.2 25);
	}

	.footer-status-text--muted {
		color: oklch(0.709 0.01 56.259 / 50%);
		font-size: 0.75rem;
		font-family: ui-monospace, monospace;
		overflow: hidden;
		text-overflow: ellipsis;
		white-space: nowrap;
	}

	.footer-status-loading {
		font-size: 0.8125rem;
		color: oklch(0.709 0.01 56.259 / 40%);
		font-style: italic;
		margin: 0;
	}

	/* ── Bottom bar ─────────────────────────────────────────────────── */
	.footer-bottom {
		max-width: 1400px;
		margin: 0 auto;
		padding: 0.875rem 2rem;
		border-top: 1px solid oklch(1 0 0 / 6%);
		display: flex;
		align-items: center;
		gap: 0.5rem;
		font-size: 0.75rem;
		color: oklch(0.709 0.01 56.259 / 40%);
	}

	.footer-sep {
		opacity: 0.5;
	}

	/* ── Responsive ─────────────────────────────────────────────────── */
	@media (max-width: 768px) {
		.footer-inner {
			grid-template-columns: 1fr 1fr;
			padding: 1.5rem 1rem 1rem;
			gap: 1.75rem;
		}

		.footer-col:first-child {
			grid-column: 1 / -1;
		}
	}

	@media (max-width: 480px) {
		.footer-inner {
			grid-template-columns: 1fr;
		}

		.footer-bottom {
			padding: 0.75rem 1rem;
		}
	}
</style>
