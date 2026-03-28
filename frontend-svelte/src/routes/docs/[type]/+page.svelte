<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { api } from '$lib/api/client';
	import { auth as authStore } from '$lib/stores/auth.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';

	const auth = authStore;

	type DocType = 'user' | 'admin' | 'cloud-init' | 'proxmox-permissions';

	const ALLOWED: DocType[] = ['user', 'admin', 'cloud-init', 'proxmox-permissions'];
	const ADMIN_ONLY: DocType[] = ['admin', 'proxmox-permissions'];

	let docType = $derived($page.params.type as DocType);
	let loading = $state(true);
	let error = $state<Error | null>(null);
	let html = $state('');
	let toc = $state<{ id: string; text: string; level: number }[]>([]);

	async function load(type: DocType) {
		if (!ALLOWED.includes(type)) {
			error = new Error('Invalid doc type');
			loading = false;
			return;
		}
		loading = true;
		error = null;
		html = '';
		toc = [];
		try {
			const res = await api.get<{ html: string }>(`/api/v1/docs/${type}`);
			html = res.html;
			toc = extractTOC(html);
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	function extractTOC(rawHtml: string): { id: string; text: string; level: number }[] {
		const entries: { id: string; text: string; level: number }[] = [];
		const re = /<h([23])[^>]*id="([^"]+)"[^>]*>(.*?)<\/h[23]>/gi;
		let m: RegExpExecArray | null;
		while ((m = re.exec(rawHtml)) !== null) {
			entries.push({ level: parseInt(m[1]), id: m[2], text: m[3].replace(/<[^>]+>/g, '') });
		}
		return entries;
	}

	function scrollTo(id: string) {
		document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	$effect(() => {
		if (docType) load(docType);
	});

	$effect(() => {
		if (auth.initialized && !auth.isAdmin && ADMIN_ONLY.includes(docType)) {
			goto('/docs/user', { replaceState: true });
		}
	});
</script>

<div class="mx-auto flex max-w-6xl gap-6 px-4 py-6">
	<!-- Sidebar -->
	<aside class="hidden w-56 shrink-0 lg:block">
		<nav class="mb-6 space-y-1">
			<a href="/docs/user" class="pv-doc-nav {docType === 'user' ? 'pv-doc-nav--active' : ''}">
				{$t('docs.user')}
			</a>
			{#if auth.isAdmin}
				<a href="/docs/admin" class="pv-doc-nav {docType === 'admin' ? 'pv-doc-nav--active' : ''}">
					{$t('docs.admin')}
				</a>
				<a
					href="/docs/cloud-init"
					class="pv-doc-nav {docType === 'cloud-init' ? 'pv-doc-nav--active' : ''}"
				>
					{$t('docs.cloudInit')}
				</a>
				<a
					href="/docs/proxmox-permissions"
					class="pv-doc-nav {docType === 'proxmox-permissions' ? 'pv-doc-nav--active' : ''}"
				>
					{$t('docs.proxmoxPerms')}
				</a>
			{/if}
		</nav>

		{#if toc.length > 0}
			<div class="border-t border-border pt-4">
				<p class="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
					{$t('docs.onThisPage')}
				</p>
				<ul class="space-y-1">
					{#each toc as entry (entry.id)}
						<li>
							<button
								class="w-full truncate text-left text-sm text-muted-foreground hover:text-foreground {entry.level === 3 ? 'pl-3' : ''}"
								onclick={() => scrollTo(entry.id)}
							>
								{entry.text}
							</button>
						</li>
					{/each}
				</ul>
			</div>
		{/if}
	</aside>

	<!-- Content -->
	<main class="min-w-0 flex-1">
		{#if error}
			<ErrorBanner {error} onRetry={() => load(docType)} />
		{:else if loading}
			<LoadingSkeleton variant="card" rows={8} />
		{:else}
			<article class="pv-prose">
				{@html html}
			</article>
		{/if}
	</main>
</div>

<style>
	:global(.pv-doc-nav) {
		display: block;
		padding: 0.375rem 0.625rem;
		border-radius: 0.375rem;
		font-size: 0.875rem;
		color: var(--muted-foreground);
		text-decoration: none;
		transition: background 0.1s, color 0.1s;
	}
	:global(.pv-doc-nav:hover) {
		background: var(--accent);
		color: var(--accent-foreground);
	}
	:global(.pv-doc-nav--active) {
		background: var(--accent);
		color: var(--accent-foreground);
		font-weight: 500;
	}
	:global(.pv-prose) {
		color: var(--foreground);
		line-height: 1.75;
		max-width: 72ch;
	}
	:global(.pv-prose h1) {
		font-size: 1.75rem;
		font-weight: 700;
		margin-bottom: 1rem;
		margin-top: 0;
		border-bottom: 1px solid var(--border);
		padding-bottom: 0.5rem;
	}
	:global(.pv-prose h2) {
		font-size: 1.25rem;
		font-weight: 600;
		margin-top: 2rem;
		margin-bottom: 0.75rem;
		border-bottom: 1px solid var(--border);
		padding-bottom: 0.25rem;
	}
	:global(.pv-prose h3) {
		font-size: 1.05rem;
		font-weight: 600;
		margin-top: 1.5rem;
		margin-bottom: 0.5rem;
	}
	:global(.pv-prose h4) {
		font-size: 0.95rem;
		font-weight: 600;
		margin-top: 1.25rem;
		margin-bottom: 0.4rem;
	}
	:global(.pv-prose p) { margin-bottom: 1rem; }
	:global(.pv-prose ul),
	:global(.pv-prose ol) { margin-bottom: 1rem; padding-left: 1.5rem; }
	:global(.pv-prose li) { margin-bottom: 0.25rem; }
	:global(.pv-prose a) {
		color: var(--primary);
		text-decoration: underline;
		text-underline-offset: 2px;
	}
	:global(.pv-prose a:hover) { opacity: 0.8; }
	:global(.pv-prose code) {
		background: var(--muted);
		border-radius: 0.25rem;
		padding: 0.1em 0.35em;
		font-size: 0.875em;
		font-family: ui-monospace, monospace;
	}
	:global(.pv-prose pre) {
		background: var(--muted);
		border-radius: 0.5rem;
		padding: 1rem;
		overflow-x: auto;
		margin-bottom: 1rem;
	}
	:global(.pv-prose pre code) { background: none; padding: 0; font-size: 0.85em; }
	:global(.pv-prose blockquote) {
		border-left: 3px solid var(--border);
		padding-left: 1rem;
		color: var(--muted-foreground);
		margin: 1rem 0;
	}
	:global(.pv-prose table) {
		width: 100%;
		border-collapse: collapse;
		margin-bottom: 1rem;
		font-size: 0.9rem;
	}
	:global(.pv-prose th) {
		text-align: left;
		padding: 0.5rem 0.75rem;
		background: var(--muted);
		border: 1px solid var(--border);
		font-weight: 600;
	}
	:global(.pv-prose td) { padding: 0.5rem 0.75rem; border: 1px solid var(--border); }
	:global(.pv-prose hr) { border: none; border-top: 1px solid var(--border); margin: 2rem 0; }
	:global(.pv-prose strong) { font-weight: 600; }
</style>
