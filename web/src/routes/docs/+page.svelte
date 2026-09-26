<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { get, ApiRequestError } from '$lib/shared/api/client';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { getLocaleContext } from '$lib/features/chrome/locale.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { Locale } from '$lib/paraglide/runtime.js';
	import { fetchDocPage, type DocSummary } from '$lib/features/docs/docs.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Select from '$lib/shared/ui/Select.svelte';

	// Help & guides (DESIGN.md §6.5): the seeded documentation as a stack of
	// numbered accordions (first one open), each body fetched when opened,
	// with a short "new to virtual machines?" aside. /docs/[id] stays the
	// deep-linkable single page.

	const session = getSessionContext();
	const locale = getLocaleContext();

	let pages = $state<DocSummary[]>([]);
	let loading = $state(true);
	let error = $state<string | null>(null);
	let selectedLang = $state<Locale>('en');

	async function loadDocs(): Promise<void> {
		loading = true;
		error = null;
		try {
			pages = await get<DocSummary[]>('/api/v1/docs');
		} catch (err) {
			error = err instanceof ApiRequestError ? err.message : m['docs.failedLoad']();
		} finally {
			loading = false;
		}
	}

	onMount(() => {
		selectedLang = locale.current;
		void loadDocs();
	});

	function visiblePages(pages: DocSummary[], isAdmin: boolean, lang: Locale): DocSummary[] {
		return pages.filter((p) => p.audience !== 'admin' || isAdmin).filter((p) => p.lang === lang);
	}

	/** Rendered bodies, fetched on first open, keyed by id + lang. */
	let bodies = $state<Record<string, { html?: string; error?: boolean }>>({});

	function bodyKey(page: DocSummary): string {
		return `${page.id}:${page.lang}`;
	}

	async function loadBody(page: DocSummary): Promise<void> {
		const key = bodyKey(page);
		if (bodies[key]?.html !== undefined) return;
		try {
			const doc = await fetchDocPage(page.id, page.lang);
			bodies = { ...bodies, [key]: { html: doc.html } };
		} catch {
			bodies = { ...bodies, [key]: { error: true } };
		}
	}

	function onToggle(event: Event, page: DocSummary): void {
		if ((event.currentTarget as HTMLDetailsElement).open) void loadBody(page);
	}

	const visible = $derived(visiblePages(pages, session.isAdmin, selectedLang));

	// The first article is open by default; load its body as soon as the
	// list is known.
	$effect(() => {
		const first = visible[0];
		if (first) void loadBody(first);
	});

	function number(index: number): string {
		return String(index + 1).padStart(2, '0');
	}
</script>

<svelte:head>
	<title>{m['docs.index']()} - PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl">
	<PageHeader
		eyebrow={m['help.eyebrow']()}
		title={m['help.heading']()}
		description={m['help.description']()}
		focusTarget
		divider={false}
	>
		{#snippet actions()}
			<label class="flex items-center gap-2 text-sm">
				{m['docs.language']()}
				<Select
					value={selectedLang}
					options={['en', 'fr']}
					onchange={(e: Event) => (selectedLang = (e.currentTarget as HTMLSelectElement).value as Locale)}
					class="w-auto"
				/>
			</label>
		{/snippet}
	</PageHeader>

	<div class="grid items-start gap-6 min-[900px]:grid-cols-[minmax(0,1fr)_260px]">
		<div>
			{#if loading}
				<p role="status" aria-live="polite" class="text-muted-foreground">{m['docs.loading']()}</p>
			{:else if error}
				<Alert>{error}</Alert>
			{:else if visible.length === 0}
				<p class="text-muted-foreground">{m['docs.empty']()}</p>
			{:else}
				<div class="grid gap-3" data-testid="help-articles">
					{#each visible as page, index (page.id + '-' + page.lang)}
						{@const body = bodies[bodyKey(page)]}
						<details
							class="group rounded-xl border border-border bg-card shadow-card"
							open={index === 0}
							ontoggle={(event) => onToggle(event, page)}
							data-testid="help-article"
						>
							<summary class="pv-focus flex cursor-pointer list-none items-center gap-3 rounded-xl px-5 py-4 [&::-webkit-details-marker]:hidden">
								<span class="font-mono text-sm text-muted-foreground tabular-nums">{number(index)}</span>
								<span class="text-muted-foreground-subtle" aria-hidden="true">-</span>
								<span class="flex-1 font-medium">{page.title}</span>
								{#if page.audience === 'admin'}
									<span class="rounded bg-destructive/10 px-2 py-0.5 text-xs font-medium text-destructive">{m['docs.audienceAdmin']()}</span>
								{/if}
								<span class="w-4 text-center text-lg leading-none text-muted-foreground group-open:hidden" aria-hidden="true">+</span>
								<span class="hidden w-4 text-center text-lg leading-none text-muted-foreground group-open:inline" aria-hidden="true">&minus;</span>
							</summary>
							<div class="border-t border-border px-5 py-4">
								{#if body?.html !== undefined}
									<article class="prose prose-sm max-w-none dark:prose-invert">
										<!-- eslint-disable-next-line svelte/no-at-html-tags -- backend renderer is XSS-safe (server/internal/httpapi/docs.go) -->
										{@html body.html}
									</article>
								{:else if body?.error}
									<p class="text-sm text-destructive">{m['help.articleError']()}</p>
								{:else}
									<p role="status" class="text-sm text-muted-foreground">{m['help.loadingArticle']()}</p>
								{/if}
								<a
									href={resolve(`/docs/${page.id}?lang=${page.lang}`)}
									class="pv-focus mt-4 inline-block rounded text-sm font-medium text-primary underline-offset-2 hover:underline"
									data-testid="help-open-page"
								>
									{m['help.openPage']()}
								</a>
							</div>
						</details>
					{/each}
				</div>
			{/if}
		</div>

		<aside class="rounded-xl border border-border bg-muted/40 p-5 text-sm" data-testid="help-aside">
			<h2 class="font-semibold">{m['help.aside.title']()}</h2>
			<p class="mt-2 text-muted-foreground">{m['help.aside.body']()}</p>
			<ul class="mt-3 grid list-disc gap-1.5 pl-5 text-muted-foreground">
				<li>{m['help.aside.conceptSize']()}</li>
				<li>{m['help.aside.conceptSsh']()}</li>
				<li>{m['help.aside.conceptConsole']()}</li>
				<li>{m['help.aside.conceptAllowance']()}</li>
			</ul>
		</aside>
	</div>
</section>
