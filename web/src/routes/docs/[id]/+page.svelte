<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { fetchDocPage, type DocRendered } from '$lib/features/docs/docs.svelte';
	import { getLocaleContext } from '$lib/features/chrome/locale.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { Locale } from '$lib/paraglide/runtime.js';
	import { ApiRequestError } from '$lib/shared/api/client';

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
		const url = new URL(page.url);
		url.searchParams.set('lang', lang);
		void goto(url, { replaceState: true, keepFocus: true, noScroll: true });
	}

	onMount(() => {
		selectedLang = langFromURL();
		void loadDoc(selectedLang);
	});
</script>

<svelte:head>
	<title>{doc ? `${doc.title} — PVMSS` : `${m['docs.title']()} — PVMSS`}</title>
</svelte:head>

<section class="mx-auto w-full max-w-3xl px-4 py-8 md:px-6">
	<a
		href={resolve('/docs')}
		class="mb-4 inline-flex items-center text-sm text-muted-foreground hover:text-foreground"
	>
		← {m['docs.back']()}
	</a>

	<div class="mb-6 flex items-center justify-between gap-4">
		{#if doc}
			<h1 class="text-2xl font-semibold">{doc.title}</h1>
		{:else}
			<h1 class="text-2xl font-semibold">{m['docs.title']()}</h1>
		{/if}
		<label class="flex items-center gap-2 text-sm">
			<span class="text-muted-foreground">{m['docs.language']()}</span>
			<select
				class="rounded-md border border-border bg-background px-2 py-1 text-sm"
				value={selectedLang}
				onchange={(e) => {
					selectedLang = e.currentTarget.value as Locale;
					updateURLLang(selectedLang);
					void loadDoc(selectedLang);
				}}
			>
				<option value="en">en</option>
				<option value="fr">fr</option>
			</select>
		</label>
	</div>

	{#if loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">{m['docs.loading']()}</p>
	{:else if notFound}
		<p role="alert" class="text-muted-foreground">{m['docs.notFound']()}</p>
	{:else if error}
		<p role="alert" class="text-destructive">{error}</p>
	{:else if doc}
		<article class="prose prose-sm max-w-none dark:prose-invert">
			<!-- eslint-disable-next-line svelte/no-at-html-tags -- backend renderer is XSS-safe (server/internal/httpapi/docs.go) -->
			{@html doc.html}
		</article>
	{/if}
</section>
