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
	const ACTIVE_SECTION_THRESHOLD = 150; // px from top to consider section active

	let docType = $derived($page.params.type as DocType);
	let loading = $state(true);
	let error = $state<Error | null>(null);
	let html = $state('');
	let toc = $state<{ id: string; text: string; level: number }[]>([]);
	let scrollProgress = $state(0);
	let activeSection = $state('');
	let sectionElements = $state<Map<string, HTMLElement>>(new Map());
	let observer: IntersectionObserver | null = null;
	let previousDocType = $state('');

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
		sectionElements.clear();
		try {
			const res = await api.get<{ html: string }>(`/api/v1/docs/${type}`);
			console.log('Docs API response:', res);
			html = res.html;
			console.log('HTML content length:', html.length);
			console.log('HTML content preview:', html.substring(0, 200));
			console.log('html state after assignment:', html.length);
			toc = extractTOC(html);
			console.log('TOC entries:', toc);
			// Wait for DOM to update before caching elements
			await new Promise((resolve) => setTimeout(resolve, 0));
			console.log('After setTimeout, html length:', html.length);
			cacheSectionElements();
			setupIntersectionObserver();
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

	function cacheSectionElements() {
		const newMap = new Map<string, HTMLElement>();
		for (const entry of toc) {
			const el = document.getElementById(entry.id);
			if (el) {
				newMap.set(entry.id, el);
			}
		}
		sectionElements = newMap;
	}

	function setupIntersectionObserver() {
		if (observer) {
			observer.disconnect();
		}

		observer = new IntersectionObserver(
			(entries) => {
				for (const entry of entries) {
					if (entry.isIntersecting && entry.target.id) {
						activeSection = entry.target.id;
					}
				}
			},
			{
				rootMargin: `-${ACTIVE_SECTION_THRESHOLD}px 0px -80% 0px`,
				threshold: 0
			}
		);

		for (const [id, element] of sectionElements) {
			observer.observe(element);
		}
	}

	function scrollTo(id: string) {
		document.getElementById(id)?.scrollIntoView({ behavior: 'smooth', block: 'start' });
	}

	function handleScroll() {
		const scrollTop = window.scrollY;
		const docHeight = document.documentElement.scrollHeight - window.innerHeight;
		scrollProgress = docHeight > 0 ? (scrollTop / docHeight) * 100 : 0;
	}

	onMount(() => {
		window.addEventListener('scroll', handleScroll, { passive: true });
		return () => {
			window.removeEventListener('scroll', handleScroll);
			if (observer) {
				observer.disconnect();
			}
		};
	});

	// Only load when docType actually changes
	$effect(() => {
		if (docType && docType !== previousDocType) {
			previousDocType = docType;
			load(docType);
		}
	});

	$effect(() => {
		if (auth.initialized && !auth.isAdmin && ADMIN_ONLY.includes(docType)) {
			goto('/docs/user', { replaceState: true });
		}
	});
</script>

<!-- Scroll Progress Bar -->
<div class="pv-scroll-progress-bar" style="width: {scrollProgress}%"></div>

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
			<div class="pv-toc-container">
				<p class="mb-2 text-xs font-semibold uppercase tracking-wider text-muted-foreground">
					{$t('docs.onThisPage')}
				</p>
				<ul class="space-y-0.5">
					{#each toc as entry (entry.id)}
						<li>
							<button
								class="pv-toc-entry {entry.level === 3 ? 'pv-toc-entry--sub' : ''} {activeSection === entry.id ? 'pv-toc-entry--active' : ''}"
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
			<!-- DEBUG: html length = {html.length} -->
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
	:global(.pv-toc-entry) {
		display: block;
		width: 100%;
		text-align: left;
		padding: 0.2rem 0.5rem;
		border-radius: 0.25rem;
		font-size: 0.8125rem;
		line-height: 1.4;
		color: var(--muted-foreground);
		background: none;
		border: none;
		cursor: pointer;
		transition: color 0.1s, background 0.1s;
		white-space: normal;
		word-break: break-words;
	}
	:global(.pv-toc-entry:hover) {
		color: var(--foreground);
		background: var(--accent);
	}
	:global(.pv-toc-entry--sub) {
		padding-left: 1rem;
		font-size: 0.75rem;
		opacity: 0.75;
	}
	:global(.pv-prose) {
		color: var(--foreground);
		line-height: 1.75;
		max-width: 72ch;
		text-rendering: optimizeLegibility;
		-webkit-font-smoothing: antialiased;
		font-variant-ligatures: common-ligatures;
	}
	:global(.pv-prose h1) {
		font-size: 1.875rem;
		font-weight: 700;
		letter-spacing: -0.02em;
		margin-bottom: 1.25rem;
		margin-top: 0;
		border-bottom: 2px solid var(--border);
		padding-bottom: 0.625rem;
		color: var(--foreground);
		scroll-margin-top: 6rem;
	}
	:global(.pv-prose h2) {
		font-size: 1.3rem;
		font-weight: 600;
		letter-spacing: -0.01em;
		margin-top: 2.5rem;
		margin-bottom: 0.875rem;
		border-bottom: 1px solid var(--border);
		padding-bottom: 0.375rem;
		color: var(--foreground);
		scroll-margin-top: 6rem;
	}
	:global(.pv-prose h3) {
		font-size: 1.1rem;
		font-weight: 600;
		margin-top: 1.75rem;
		margin-bottom: 0.5rem;
		color: var(--foreground);
		scroll-margin-top: 6rem;
	}
	:global(.pv-prose h4) {
		font-size: 0.975rem;
		font-weight: 600;
		margin-top: 1.25rem;
		margin-bottom: 0.4rem;
		color: var(--muted-foreground);
		text-transform: uppercase;
		letter-spacing: 0.04em;
		font-size: 0.8rem;
	}
	:global(.pv-prose p) { margin-bottom: 1rem; }
	:global(.pv-prose ul),
	:global(.pv-prose ol) { margin-bottom: 1rem; padding-left: 1.75rem; }
	:global(.pv-prose li) { margin-bottom: 0.375rem; }
	:global(.pv-prose li > ul),
	:global(.pv-prose li > ol) { margin-top: 0.25rem; margin-bottom: 0.25rem; }
	:global(.pv-prose a) {
		color: var(--primary);
		text-decoration: underline;
		text-underline-offset: 3px;
		text-decoration-thickness: 1px;
		transition: opacity 0.15s;
	}
	:global(.pv-prose a:hover) { opacity: 0.75; }
	:global(.pv-prose code) {
		background: var(--muted);
		border: 1px solid var(--border);
		border-radius: 0.3rem;
		padding: 0.15em 0.4em;
		font-size: 0.85em;
		font-family: ui-monospace, 'Cascadia Code', 'Fira Mono', monospace;
		color: var(--foreground);
	}
	:global(.pv-prose pre) {
		background: var(--muted);
		border: 1px solid var(--border);
		border-radius: 0.5rem;
		padding: 1.125rem 1.25rem;
		overflow-x: auto;
		margin-bottom: 1.25rem;
		line-height: 1.6;
	}
	:global(.pv-prose pre code) {
		background: none;
		border: none;
		padding: 0;
		font-size: 0.875em;
	}
	:global(.pv-prose blockquote) {
		border-left: 3px solid var(--primary);
		padding: 0.5rem 1rem;
		color: var(--muted-foreground);
		margin: 1.25rem 0;
		background: var(--muted);
		border-radius: 0 0.375rem 0.375rem 0;
		font-style: italic;
	}
	:global(.pv-prose blockquote p:last-child) { margin-bottom: 0; }
	:global(.pv-prose table) {
		width: 100%;
		border-collapse: collapse;
		margin-bottom: 1.25rem;
		font-size: 0.9rem;
		border: 1px solid var(--border);
		border-radius: 0.5rem;
		overflow: hidden;
	}
	:global(.pv-prose th) {
		text-align: left;
		padding: 0.625rem 0.875rem;
		background: var(--muted);
		border-bottom: 2px solid var(--border);
		font-weight: 600;
		font-size: 0.825rem;
		text-transform: uppercase;
		letter-spacing: 0.04em;
		color: var(--muted-foreground);
	}
	:global(.pv-prose td) {
		padding: 0.5rem 0.875rem;
		border-bottom: 1px solid var(--border);
		vertical-align: top;
	}
	:global(.pv-prose tr:last-child td) { border-bottom: none; }
	:global(.pv-prose tr:nth-child(even) td) {
		background: var(--muted);
	}
	:global(.pv-prose hr) { border: none; border-top: 1px solid var(--border); margin: 2.5rem 0; }
	:global(.pv-prose strong) { font-weight: 600; color: var(--foreground); }

	/* Scroll Progress Bar */
	:global(.pv-scroll-progress-bar) {
		position: fixed;
		top: 0;
		left: 0;
		height: 3px;
		background: linear-gradient(90deg, var(--primary), var(--primary));
		z-index: 9999;
		transition: width 0.1s ease-out;
		box-shadow: 0 0 10px rgba(var(--primary), 0.3);
	}

	/* Sticky TOC Container */
	:global(.pv-toc-container) {
		position: sticky;
		top: 5.5rem;
		max-height: calc(100vh - 7rem);
		overflow-y: auto;
		padding-right: 0.5rem;
		scrollbar-width: thin;
		scrollbar-color: var(--muted-foreground) var(--muted);
	}

	:global(.pv-toc-container::-webkit-scrollbar) {
		width: 6px;
	}

	:global(.pv-toc-container::-webkit-scrollbar-track) {
		background: var(--muted);
		border-radius: 3px;
	}

	:global(.pv-toc-container::-webkit-scrollbar-thumb) {
		background: var(--muted-foreground);
		border-radius: 3px;
	}

	:global(.pv-toc-container::-webkit-scrollbar-thumb:hover) {
		background: var(--foreground);
	}

	/* Active TOC Entry */
	:global(.pv-toc-entry--active) {
		color: var(--primary) !important;
		font-weight: 600;
		background: var(--accent) !important;
		border-left: 2px solid var(--primary);
	}
</style>
