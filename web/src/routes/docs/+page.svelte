<script lang="ts">
	import { onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import { get, ApiRequestError } from '$lib/shared/api/client';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { getLocaleContext } from '$lib/features/chrome/locale.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import type { Locale } from '$lib/paraglide/runtime.js';
	import type { DocSummary } from '$lib/features/docs/docs.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import Card from '$lib/shared/ui/Card.svelte';

	interface CategoryGroup {
		category: string;
		pages: DocSummary[];
	}

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

	function visiblePages(pages: DocSummary[], isAdmin: boolean, lang: Locale): DocSummary[] {
		return pages.filter((p) => p.audience !== 'admin' || isAdmin).filter((p) => p.lang === lang);
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

	{#if loading}
		<p role="status" aria-live="polite" class="text-muted-foreground">{m['docs.loading']()}</p>
	{:else if error}
		<Alert>{error}</Alert>
	{:else}
		{@const grouped = groupByCategory(visiblePages(pages, session.isAdmin, selectedLang))}
		{#if grouped.length === 0}
			<p class="text-muted-foreground">{m['docs.empty']()}</p>
		{:else}
			{#each grouped as group (group.category)}
				<section class="mb-8">
					<h2 class="mb-3 text-lg font-medium">{group.category}</h2>
					<Card as="div" pad="none">
						<ul class="divide-y divide-border">
							{#each group.pages as page, index (page.id + '-' + page.lang)}
								<li>
									<a
										href={resolve(`/docs/${page.id}?lang=${page.lang}`)}
										class="flex items-center justify-between gap-4 px-4 py-3 transition-colors hover:bg-accent/30 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
										class:rounded-t-xl={index === 0}
										class:rounded-b-xl={index === group.pages.length - 1}
									>
										<div class="flex min-w-0 flex-1 flex-col gap-0.5">
											<div class="flex items-center gap-2">
												<h3 class="truncate font-medium">{page.title}</h3>
												{#if page.audience === 'admin'}
													<span class={`shrink-0 rounded px-2 py-0.5 text-xs font-medium ${audienceBadgeClass(page.audience)}`}>
														{m['docs.audienceAdmin']()}
													</span>
												{/if}
											</div>
											<p class="text-xs text-muted-foreground">
												<span class="font-mono">{page.id}</span> · {page.lang}
											</p>
										</div>
									</a>
								</li>
							{/each}
						</ul>
					</Card>
				</section>
			{/each}
		{/if}
	{/if}
</section>
