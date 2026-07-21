<script lang="ts">
	import { page } from '$app/stores';
	import { SvelteMap } from 'svelte/reactivity';
	import { goto } from '$app/navigation';
	import { t, locale } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import DOMPurify from 'dompurify';
	import { api } from '$lib/api/client';
	import { auth as authStore } from '$lib/stores/auth.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import * as Sheet from '$lib/components/ui/sheet';
	import {
		ListIcon,
		XIcon,
		MagnifyingGlassIcon,
		ArrowUpIcon,
		ClockIcon
	} from 'phosphor-svelte';

	const auth = authStore;

	type DocType = 'user' | 'admin' | 'cloud-init' | 'proxmox-permissions';

	const ALLOWED: DocType[] = ['user', 'admin', 'cloud-init', 'proxmox-permissions'];
	const ADMIN_ONLY: DocType[] = ['admin', 'proxmox-permissions'];
	const ACTIVE_SECTION_THRESHOLD = 150; // px from top to consider section active

	/**
	 * Decode HTML entities (e.g. &rsquo; → ’, &amp; → &) for plain-text use.
	 * Used for TOC labels so users see real characters instead of entities.
	 */
	function decodeHtmlEntities(str: string): string {
		if (typeof document === 'undefined') {
			// Fallback for any non-DOM context (defensive)
			return str
				.replace(/&amp;/g, '&')
				.replace(/&lt;/g, '<')
				.replace(/&gt;/g, '>')
				.replace(/&quot;/g, '"')
				.replace(/&#39;/g, "'")
				.replace(/&apos;/g, "'")
				.replace(/&rsquo;/g, '’')
				.replace(/&lsquo;/g, '‘')
				.replace(/&ldquo;/g, '“')
				.replace(/&rdquo;/g, '”')
				.replace(/&ndash;/g, '–')
				.replace(/&mdash;/g, '—')
				.replace(/&hellip;/g, '…');
		}
		const textarea = document.createElement('textarea');
		textarea.innerHTML = str;
		return textarea.value;
	}

	let docType = $derived($page.params.type as DocType);
	let loading = $state(true);
	let error = $state<Error | null>(null);
	let html = $state('');
	let toc = $state<{ id: string; text: string; level: number }[]>([]);
	let scrollProgress = $state(0);
	let activeSection = $state('');
	const sectionElements = new SvelteMap<string, HTMLElement>();
	let observer: IntersectionObserver | null = null;
	let previousDocType = $state('');

	// UX enhancements
	let mobileTocOpen = $state(false);
	let searchQuery = $state('');
	let readingTime = $state(0);
	let backToTopVisible = $state(false);

	const filteredToc = $derived(
		searchQuery.trim().length > 0
			? toc.filter((e) => e.text.toLowerCase().includes(searchQuery.toLowerCase()))
			: toc
	);

	function computeReadingTime(content: string): number {
		const text = content.replace(/<[^>]+>/g, ' ').replace(/\s+/g, ' ').trim();
		const words = text ? text.split(' ').length : 0;
		return Math.max(1, Math.round(words / 200));
	}

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
		searchQuery = '';
		try {
			const lang = ($locale ?? 'en').startsWith('fr') ? 'fr' : 'en';
			const res = await api.get<{ html: string }>(`/api/v1/docs/${type}?lang=${lang}`);
			// Sanitize backend-rendered HTML before {@html} injection (XSS defense-in-depth)
			html = DOMPurify.sanitize(res.html);
			toc = extractTOC(html);
			readingTime = computeReadingTime(html);
			// Wait for DOM update then enhance + cache
			await new Promise((resolve) => setTimeout(resolve, 0));
			cacheSectionElements();
			setupIntersectionObserver();
			enhanceProseContent();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	function enhanceProseContent() {
		const root = document.querySelector('.pv-prose');
		if (!root) return;

		// Heading permalinks (h2/h3)
		const headings = root.querySelectorAll('h2[id], h3[id]');
		headings.forEach((h) => {
			const el = h as HTMLElement;
			if (el.querySelector('.pv-heading-anchor')) return;
			const anchor = document.createElement('a');
			anchor.href = `#${el.id}`;
			anchor.className = 'pv-heading-anchor';
			anchor.setAttribute('aria-label', 'Link to section');
			anchor.innerHTML = '<span aria-hidden="true">#</span>';
			anchor.addEventListener('click', (e) => {
				e.preventDefault();
				const url = `${location.pathname}${location.search}#${el.id}`;
				history.replaceState(null, '', `#${el.id}`);
				navigator.clipboard.writeText(window.location.origin + url).then(() => {
					toast.success($t('common.copied', { values: { value: 'link' } }));
				});
				// Use the centralized scrollTo so we get correct offset for fixed navbar + sticky header + breathing room
				scrollTo(el.id);
			});
			el.appendChild(anchor);
		});

		// Code copy buttons for pre blocks
		const pres = root.querySelectorAll('pre');
		pres.forEach((pre) => {
			const p = pre as HTMLElement;
			if (p.querySelector('.pv-code-copy')) return;
			const btn = document.createElement('button');
			btn.type = 'button';
			btn.className = 'pv-code-copy';
			btn.setAttribute('aria-label', $t('docs.copyCode'));
			btn.innerHTML = '<span aria-hidden="true">⎘</span>';
			btn.addEventListener('click', async () => {
				const code = p.querySelector('code')?.textContent ?? p.textContent ?? '';
				try {
					await navigator.clipboard.writeText(code);
					const orig = btn.innerHTML;
					btn.innerHTML = '✓';
					toast.success($t('docs.copiedCode'));
					setTimeout(() => {
						btn.innerHTML = orig;
					}, 1200);
				} catch {
					toast.error($t('common.copyFailed'));
				}
			});
			p.style.position = 'relative';
			p.appendChild(btn);
		});
	}

	function scrollTo(id: string) {
		const el = document.getElementById(id);
		if (!el) return;

		// Close mobile drawer if open
		mobileTocOpen = false;

		// Measure the two bars that can cover the heading:
		// 1. Main fixed navbar (.pv-navbar, height ~56px)
		// 2. Sticky docs header (the <header class="sticky top-[56px]">)
		const navbar = document.querySelector('.pv-navbar') as HTMLElement | null;
		const stickyHeader = document.querySelector('header.sticky') as HTMLElement | null;

		let offset = 0;
		if (navbar) offset += navbar.offsetHeight || 56;
		if (stickyHeader) offset += stickyHeader.offsetHeight || 52;

		// Extra visible padding/space above the heading text (and its orange underline)
		const extra = 72; // generous breathing room so the heading is not tight against the navbars

		const rect = el.getBoundingClientRect();
		const absoluteTop = rect.top + window.scrollY;
		const targetY = absoluteTop - offset - extra;

		window.scrollTo({ top: Math.max(0, targetY), behavior: 'smooth' });
	}

	function handleTocSearchKey(e: KeyboardEvent) {
		const first = filteredToc[0];
		if (e.key === 'Enter' && first) {
			scrollTo(first.id);
		}
	}

	function extractTOC(rawHtml: string): { id: string; text: string; level: number }[] {
		const entries: { id: string; text: string; level: number }[] = [];
		const re = /<h([23])[^>]*id="([^"]+)"[^>]*>(.*?)<\/h[23]>/gi;
		let m: RegExpExecArray | null;
		while ((m = re.exec(rawHtml)) !== null) {
			const [, levelStr, id, inner] = m;
			if (!levelStr || !id || inner === undefined) continue;
			const plain = inner.replace(/<[^>]+>/g, '');
			entries.push({ level: parseInt(levelStr), id, text: decodeHtmlEntities(plain) });
		}
		return entries;
	}

	function cacheSectionElements() {
		sectionElements.clear();
		for (const entry of toc) {
			const el = document.getElementById(entry.id);
			if (el) {
				sectionElements.set(entry.id, el);
			}
		}
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

		for (const [, element] of sectionElements) {
			observer.observe(element);
		}
	}

	function handleScroll() {
		const scrollTop = window.scrollY;
		const docHeight = document.documentElement.scrollHeight - window.innerHeight;
		scrollProgress = docHeight > 0 ? (scrollTop / docHeight) * 100 : 0;
		backToTopVisible = scrollTop > 400;
	}

	function backToTop() {
		window.scrollTo({ top: 0, behavior: 'smooth' });
	}

	$effect(() => {
		window.addEventListener('scroll', handleScroll, { passive: true });
		return () => {
			window.removeEventListener('scroll', handleScroll);
			observer?.disconnect();
		};
	});

	// Close mobile TOC on Escape
	$effect(() => {
		if (!mobileTocOpen) return;
		const onKey = (e: KeyboardEvent) => {
			if (e.key === 'Escape') mobileTocOpen = false;
		};
		document.addEventListener('keydown', onKey);
		return () => document.removeEventListener('keydown', onKey);
	});

	let previousLocale = $state('');

	// Load when docType or locale changes
	$effect(() => {
		const currentLocale = $locale ?? '';
		if (docType && (docType !== previousDocType || currentLocale !== previousLocale)) {
			previousDocType = docType;
			previousLocale = currentLocale;
			load(docType);
		}
	});

	$effect(() => {
		if (auth.initialized && !auth.isAdmin && ADMIN_ONLY.includes(docType)) {
			goto('/docs/user', { replaceState: true });
		}
	});
</script>

<svelte:head>
	<title>PVMSS — {$t(`docs.${docType === 'cloud-init' ? 'cloudInit' : docType === 'proxmox-permissions' ? 'proxmoxPerms' : docType}`)}</title>
</svelte:head>

<!-- Scroll Progress Bar -->
<div class="pv-scroll-progress-bar" style="width: {scrollProgress}%"></div>

<!-- Docs Header -->
<header class="sticky top-[56px] z-40 border-b border-border bg-background/95 backdrop-blur supports-[backdrop-filter]:bg-background/80">
	<div class="mx-auto flex items-center justify-between gap-3 px-4 py-3 max-w-4xl">
		<div class="flex min-w-0 items-center gap-3">
			<div class="flex items-center gap-2">
				<span class="text-sm font-medium text-muted-foreground">{$t('docs.documentation')}</span>
				<span class="text-muted-foreground/50">/</span>
				<span class="truncate text-base font-semibold">
					{$t(`docs.${docType === 'cloud-init' ? 'cloudInit' : docType === 'proxmox-permissions' ? 'proxmoxPerms' : docType}`)}
				</span>
			</div>
			{#if readingTime > 0}
				<span class="hidden items-center gap-1 rounded-full bg-muted px-2 py-0.5 text-[11px] font-medium text-muted-foreground sm:inline-flex">
					<ClockIcon class="h-3 w-3" />
					{$t('docs.readingTime', { values: { count: readingTime } })}
				</span>
			{/if}
		</div>

		<div class="flex items-center gap-2">
			<!-- TOC trigger (works for all sizes; opens the in-page table of contents) -->
			{#if toc.length > 0}
				<button
					class="pv-toc-trigger"
					onclick={() => (mobileTocOpen = true)}
					aria-label={$t('docs.openToc')}
				>
					<ListIcon class="h-4 w-4" />
					<span class="hidden sm:inline">{$t('docs.toc')}</span>
				</button>
			{/if}
		</div>
	</div>
</header>

<div class="mx-auto px-4 py-6 max-w-4xl">
	<!-- Content (single column, no left sidebar) -->
	<main id="main-content">
		{#if error}
			<ErrorBanner {error} onRetry={() => load(docType)} />
		{:else if loading}
			<LoadingSkeleton variant="card" rows={8} />
		{:else}
			<article class="pv-prose">
				<!-- eslint-disable-next-line svelte/no-at-html-tags -- trusted app-owned docs HTML, not user input -->
				{@html html}
			</article>
		{/if}
	</main>
</div>

<!-- Mobile TOC Drawer -->
<Sheet.Root bind:open={mobileTocOpen}>
	<Sheet.Content side="right" class="w-72 p-0">
		<div class="flex h-full flex-col">
			<div class="flex items-center justify-between border-b px-4 py-3">
				<div class="flex items-center gap-2 text-sm font-semibold">
					<ListIcon class="h-4 w-4" />
					{$t('docs.toc')}
				</div>
				<button class="rounded p-1 hover:bg-accent" onclick={() => (mobileTocOpen = false)} aria-label={$t('docs.close')}>
					<XIcon class="h-4 w-4" />
				</button>
			</div>

			<div class="flex-1 overflow-auto p-3">
				{#if toc.length > 0}
					<div class="relative mb-2">
						<MagnifyingGlassIcon class="pointer-events-none absolute left-2.5 top-2 h-3.5 w-3.5 text-muted-foreground" />
						<input
							type="text"
							class="pv-toc-search"
							placeholder={$t('docs.searchInPage')}
							bind:value={searchQuery}
							onkeydown={handleTocSearchKey}
						/>
					</div>
					<ul class="space-y-0.5">
						{#each filteredToc as entry (entry.id)}
							<li>
								<button
									class="pv-toc-entry {entry.level === 3 ? 'pv-toc-entry--sub' : ''} {activeSection === entry.id ? 'pv-toc-entry--active' : ''}"
									onclick={() => scrollTo(entry.id)}
								>
									{entry.text}
								</button>
							</li>
						{/each}
						{#if searchQuery && filteredToc.length === 0}
							<li class="px-2 py-1 text-xs text-muted-foreground">{$t('common.noMatches')}</li>
						{/if}
					</ul>
				{/if}
			</div>
		</div>
	</Sheet.Content>
</Sheet.Root>

<!-- Floating Back to Top -->
{#if backToTopVisible}
	<button
		class="pv-back-to-top"
		onclick={backToTop}
		aria-label={$t('docs.backToTop')}
	>
		<ArrowUpIcon class="h-4 w-4" />
	</button>
{/if}

<style>
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
	/* Global scroll padding so fixed navbars (main navbar + sticky docs header) never hide headings on anchor navigation */
	:global(html) {
		scroll-padding-top: 12rem;
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
		font-size: 2.25rem;
		line-height: 1.15;
		font-weight: 700;
		letter-spacing: -0.03em;
		margin-top: 0;
		margin-bottom: 1.25rem;
		color: var(--foreground);
		scroll-margin-top: 12rem;
		padding-top: 1.5rem;
		border-bottom: 3px solid var(--primary);
		padding-bottom: 0.35rem;
	}
	:global(.pv-prose h2) {
		font-size: 1.65rem;
		line-height: 1.25;
		font-weight: 600;
		letter-spacing: -0.02em;
		margin-top: 2.25rem;
		margin-bottom: 0.75rem;
		color: var(--foreground);
		scroll-margin-top: 12rem;
		padding-top: 1.5rem;
		border-bottom: 2px solid var(--primary);
		padding-bottom: 0.3rem;
	}
	:global(.pv-prose h3) {
		font-size: 1.35rem;
		line-height: 1.3;
		font-weight: 600;
		margin-top: 1.75rem;
		margin-bottom: 0.5rem;
		color: var(--foreground);
		scroll-margin-top: 12rem;
		padding-top: 1.25rem;
		border-bottom: 2px solid var(--primary);
		padding-bottom: 0.25rem;
	}
	:global(.pv-prose h4) {
		font-size: 1.1rem;
		line-height: 1.35;
		font-weight: 600;
		margin-top: 1.25rem;
		margin-bottom: 0.4rem;
		color: var(--muted-foreground);
		scroll-margin-top: 12rem;
		padding-top: 0.75rem;
		border-bottom: 1px solid var(--primary);
		padding-bottom: 0.2rem;
	}
	:global(.pv-prose h5) {
		font-size: 0.95rem;
		line-height: 1.4;
		font-weight: 600;
		margin-top: 1rem;
		margin-bottom: 0.3rem;
		color: var(--muted-foreground);
		scroll-margin-top: 12rem;
		padding-top: 0.6rem;
		border-bottom: 1px solid var(--primary);
		padding-bottom: 0.15rem;
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

	/* TOC trigger button (header, works on all sizes) */
	:global(.pv-toc-trigger) {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 6px 10px;
		border-radius: 6px;
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--muted-foreground);
		background: var(--muted);
		border: 1px solid var(--border);
		transition: background 0.12s, color 0.12s, border-color 0.12s;
	}
	:global(.pv-toc-trigger:hover) {
		background: var(--accent);
		color: var(--accent-foreground);
		border-color: var(--border);
	}

	/* TOC in-page search input */
	:global(.pv-toc-search) {
		width: 100%;
		padding: 4px 8px 4px 26px;
		font-size: 0.75rem;
		line-height: 1.2;
		border: 1px solid var(--border);
		border-radius: 6px;
		background: var(--background);
		color: var(--foreground);
		outline: none;
		transition: border-color 0.12s, box-shadow 0.12s;
	}
	:global(.pv-toc-search:focus) {
		border-color: var(--ring);
		box-shadow: 0 0 0 2px color-mix(in oklab, var(--ring) 18%, transparent);
	}
	:global(.pv-toc-search::placeholder) {
		color: var(--muted-foreground);
		opacity: 0.7;
	}

	/* Floating back to top */
	:global(.pv-back-to-top) {
		position: fixed;
		bottom: 20px;
		right: 20px;
		z-index: 60;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 36px;
		height: 36px;
		border-radius: 9999px;
		background: var(--card);
		color: var(--foreground);
		border: 1px solid var(--border);
		box-shadow: 0 4px 12px rgba(0, 0, 0, 0.08);
		transition: transform 0.15s ease, box-shadow 0.15s ease, background 0.12s;
	}
	:global(.pv-back-to-top:hover) {
		transform: translateY(-1px);
		box-shadow: 0 6px 16px rgba(0, 0, 0, 0.1);
		background: var(--accent);
	}

	/* Heading permalink anchors */
	:global(.pv-prose h2 .pv-heading-anchor),
	:global(.pv-prose h3 .pv-heading-anchor) {
		opacity: 0;
		margin-left: 0.5rem;
		font-size: 0.85em;
		color: var(--muted-foreground);
		text-decoration: none;
		transition: opacity 0.12s, color 0.12s;
	}
	:global(.pv-prose h2:hover .pv-heading-anchor),
	:global(.pv-prose h3:hover .pv-heading-anchor),
	:global(.pv-prose h2:focus-within .pv-heading-anchor),
	:global(.pv-prose h3:focus-within .pv-heading-anchor) {
		opacity: 1;
	}
	:global(.pv-prose h2 .pv-heading-anchor:hover),
	:global(.pv-prose h3 .pv-heading-anchor:hover) {
		color: var(--primary);
	}

	/* Code copy button inside pre */
	:global(.pv-prose pre .pv-code-copy) {
		position: absolute;
		top: 8px;
		right: 8px;
		display: inline-flex;
		align-items: center;
		justify-content: center;
		width: 28px;
		height: 28px;
		padding: 0;
		border-radius: 6px;
		border: 1px solid var(--border);
		background: color-mix(in oklab, var(--card) 92%, transparent);
		color: var(--muted-foreground);
		font-size: 0.85rem;
		line-height: 1;
		opacity: 0.6;
		transition: opacity 0.12s, background 0.12s, color 0.12s, border-color 0.12s;
	}
	:global(.pv-prose pre:hover .pv-code-copy) {
		opacity: 1;
	}
	:global(.pv-prose pre .pv-code-copy:hover) {
		background: var(--accent);
		color: var(--accent-foreground);
		border-color: var(--border);
	}
</style>
