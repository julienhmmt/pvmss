<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { fetchDocPage, type DocRendered } from '#lib/features/docs/docs.svelte.js';
	import { getLocaleContext } from '#lib/features/chrome/locale.svelte.js';
	import { m } from '#lib/paraglide/messages.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import type { Locale } from '#lib/paraglide/runtime.js';
	import { ApiRequestError } from '#lib/shared/api/client.js';

	const id = page.params.id ?? '';
	const locale = getLocaleContext();

	let doc = $state<DocRendered | null>(null);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let notFound = $state(false);
	let selectedLang = $state<Locale>('en');

	function langFromURL(): Locale {
		const value = page.url.searchParams.get('lang');
		return value === 'en' || value === 'fr' ? value : locale.current;
	}

	async function loadDoc(lang: Locale): Promise<void> {
		loading = true;
		error = null;
		notFound = false;
		doc = null;
		try {
			doc = await fetchDocPage(id, lang);
		} catch (err) {
			if (err instanceof ApiRequestError && err.status === 404) {
				notFound = true;
			} else {
				error = err instanceof ApiRequestError ? err.message : m['docs.failedRender']();
			}
		} finally {
			loading = false;
		}
	}

	function updateURLLang(lang: Locale): void {
		const url = new URL(page.url.href);
		url.searchParams.set('lang', lang);
		void goto(url, { replace: true, reset: false });
	}

	onMount(() => {
		selectedLang = langFromURL();
		void loadDoc(selectedLang);
	});
</script>

<svelte:head>
	<title>{doc ? `${doc.title} - PVMSS` : `${m['docs.title']()} - PVMSS`}</title>
</svelte:head>

<section class="w-full max-w-reading">
	<PageHeader
		title={doc ? doc.title : m['docs.title']()}
		eyebrow={m['help.eyebrow']()}
		back={{ href: resolve('/docs'), label: m['docs.back']() }}
	>
		{#snippet actions()}
			<label class="flex items-center gap-2 text-sm">
				{m['docs.language']()}
				<Select
					value={selectedLang}
					options={['en', 'fr']}
					onchange={(e: Event) => {
						selectedLang = (e.currentTarget as HTMLSelectElement).value as Locale;
						updateURLLang(selectedLang);
						void loadDoc(selectedLang);
					}}
					class="w-auto"
				/>
			</label>
		{/snippet}
	</PageHeader>

	{#if loading}
		<div role="status" aria-live="polite" class="flex flex-col gap-3">
			<span class="sr-only">{m['docs.loading']()}</span>
			<Skeleton class="h-6 w-1/2" />
			<Skeleton class="h-4 w-full" />
			<Skeleton class="h-4 w-full" />
			<Skeleton class="h-4 w-2/3" />
		</div>
	{:else if notFound}
		<EmptyState title={m['docs.notFound']()} />
	{:else if error}
		<Alert>{error}</Alert>
	{:else if doc}
		<Card as="article" pad="lg">
			<div class="doc-content">
				<!-- eslint-disable-next-line svelte/no-at-html-tags -- backend renderer is XSS-safe (server/internal/httpapi/docs.go) -->
				{@html doc.html}
			</div>
		</Card>
	{/if}
</section>

<style>
	.doc-content :global(h2),
	.doc-content :global(h3),
	.doc-content :global(h4),
	.doc-content :global(h5),
	.doc-content :global(h6) {
		color: var(--foreground);
		font-weight: 600;
		line-height: 1.25;
		margin-bottom: 0.5rem;
		margin-top: 1.5rem;
	}

	/* The renderer emits h2-h4: the page header owns the h1. */
	.doc-content :global(h2) {
		font-size: 1.25rem;
	}

	.doc-content :global(h3) {
		font-size: 1.125rem;
	}

	.doc-content :global(h4) {
		font-size: 1rem;
	}

	.doc-content :global(p) {
		margin-bottom: 1rem;
		line-height: 1.5;
	}

	.doc-content :global(ul),
	.doc-content :global(ol) {
		margin-bottom: 1rem;
		padding-left: 1.25rem;
	}

	.doc-content :global(li) {
		margin-bottom: 0.25rem;
	}

	.doc-content :global(a) {
		color: var(--primary);
		text-decoration: underline;
		text-underline-offset: 2px;
	}

	.doc-content :global(a):hover {
		text-decoration: none;
	}

	.doc-content :global(code) {
		font-family: var(--font-mono);
		font-size: 0.875rem;
		background-color: var(--muted);
		padding: 0.125rem 0.25rem;
		border-radius: 0.25rem;
	}

	.doc-content :global(pre) {
		background-color: var(--muted);
		padding: 1rem;
		border-radius: var(--radius);
		overflow-x: auto;
		margin-bottom: 1rem;
	}

	.doc-content :global(pre code) {
		background-color: transparent;
		padding: 0;
	}

	.doc-content :global(hr) {
		border: 0;
		border-top: 1px solid var(--border);
		margin: 1.5rem 0;
	}

	.doc-content :global(img) {
		max-width: 100%;
		height: auto;
		border-radius: var(--radius);
	}
</style>
