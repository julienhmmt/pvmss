<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { DocsBrowserStore, type DocSummary } from '$lib/features/docs/docs.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { getLocaleContext } from '$lib/features/chrome/locale.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { Locale } from '$lib/paraglide/runtime.js';

	interface CategoryGroup {
		category: string;
		pages: DocSummary[];
	}

	const session = getSessionContext();
	const locale = getLocaleContext();
	const store = new DocsBrowserStore();

	let selectedLang = $state<Locale>('en');

	onMount(() => {
		selectedLang = locale.current;
		void store.load();
	});

	function groupByCategory(pages: DocSummary[]): CategoryGroup[] {
		const categories: string[] = [];
		for (const page of pages) {
			if (!categories.includes(page.category)) {
				categories.push(page.category);
			}
		}
		return categories.map((category) => ({
			category,
			pages: pages.filter((p) => p.category === category)
		}));
	}

	function visiblePages(pages: DocSummary[], isAdmin: boolean): DocSummary[] {
		return pages.filter((p) => isAdmin || p.audience !== 'admin');
	}

	function docHref(page: DocSummary): string {
		return resolve(`/docs/${page.id}`);
	}

	function audienceBadgeClass(a: 'user' | 'admin'): string {
		return a === 'admin'
			? 'bg-destructive/10 text-destructive'
			: 'bg-primary/10 text-primary';
	}
</script>

<svelte:head>
	<title>{m['docs.index']()} — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-4xl px-4 py-8 md:px-6">
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-semibold">{m['docs.title']()}</h1>
		<label class="flex items-center gap-2 text-sm">
			<span class="text-muted-foreground">{m['docs.language']()}</span>
			<select
				class="rounded-md border border-border bg-background px-2 py-1 text-sm"
				value={selectedLang}
				onchange={(e) => (selectedLang = e.currentTarget.value as Locale)}
			>
				<option value="en">en</option>
				<option value="fr">fr</option>
			</select>
		</label>
	</div>

	{#if store.loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">{m['docs.loading']()}</p>
	{:else if store.error}
		<p role="alert" class="text-destructive">{store.error}</p>
	{:else}
		{@const grouped = groupByCategory(visiblePages(store.pages, session.isAdmin))}
		{#if grouped.length === 0}
			<p class="text-muted-foreground">{m['docs.empty']()}</p>
		{:else}
			{#each grouped as group (group.category)}
				<div class="mb-8">
					<h2 class="mb-3 text-lg font-medium">{group.category}</h2>
					<div class="grid grid-cols-1 gap-4 sm:grid-cols-2 lg:grid-cols-3">
						{#each group.pages as page (page.id)}
							<a
								href={docHref(page)}
								class="block rounded-lg border border-border p-4 transition-colors hover:border-primary/50 hover:bg-accent/30"
							>
								<div class="mb-2 flex items-center justify-between gap-2">
									<h3 class="font-medium">{page.title}</h3>
									{#if page.audience === 'admin'}
										<span class={`shrink-0 rounded px-2 py-0.5 text-xs font-medium ${audienceBadgeClass(page.audience)}`}>
											{m['docs.audienceAdmin']()}
										</span>
									{/if}
								</div>
								<p class="text-xs text-muted-foreground">
									<span class="font-mono">{page.id}</span> · {page.lang}
								</p>
							</a>
						{/each}
					</div>
				</div>
			{/each}
		{/if}
	{/if}
</section>
