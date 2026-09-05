<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import { getSearchContext } from './search.svelte';
	import type { VmListItem, VmStatus } from '$lib/features/vms/list.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import SearchIcon from '$lib/shared/ui/icons/SearchIcon.svelte';

	const store = getSearchContext();

	const statusTone: Record<VmStatus, 'ok' | 'off' | 'warn'> = {
		running: 'ok',
		stopped: 'off',
		paused: 'warn'
	};

	const statusLabels: Record<VmStatus, () => string> = {
		running: () => m['common.statusRunning'](),
		stopped: () => m['common.statusStopped'](),
		paused: () => m['common.statusPaused']()
	};

	function vmHref(machine: VmListItem): string {
		return resolve(`/vms/${encodeURIComponent(machine.cluster)}/${machine.vmid}`);
	}

	function handleInput(event: Event): void {
		store.applySearch((event.currentTarget as HTMLInputElement).value);
	}
</script>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<h1 class="mb-6 text-2xl font-semibold tracking-tight">{m['search.heading']()}</h1>

	<div class="relative mb-6">
		<label for="global-search" class="sr-only">{m['search.label']()}</label>
		<div class="pointer-events-none absolute inset-y-0 left-0 flex items-center pl-3.5">
			<SearchIcon class="h-5 w-5 text-muted-foreground" />
		</div>
		<input
			id="global-search"
			type="search"
			placeholder={m['search.placeholder']()}
			class="pv-input w-full pl-11 text-base"
			value={store.query}
			oninput={handleInput}
			data-testid="global-search-input"
		/>
	</div>

	{#if store.loading && store.result === null}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<Card as="div" pad="none" class="overflow-hidden">
			<ul role="list" class="divide-y divide-border">
				{#each [1, 2, 3] as i (i)}
					<li class="p-4">
						<Skeleton class="mb-3 h-5 w-1/3 max-w-52" />
						<Skeleton class="mb-2 h-4 w-1/2 max-w-72" />
						<Skeleton class="h-4 w-2/3 max-w-sm" />
					</li>
				{/each}
			</ul>
		</Card>
	{:else if store.error}
		<Alert data-testid="search-error">{store.error}</Alert>
	{:else if store.result === null}
		<EmptyState title={m['search.instructions']()} dataTestid="search-instructions" />
	{:else if store.result.items.length === 0}
		<EmptyState title={m['search.empty']()} dataTestid="search-empty" />
	{:else}
		<Card as="div" pad="none" class="overflow-hidden">
			<div class="sr-only" role="status" aria-live="polite">{m['search.resultsCaption']()}</div>
			<ul role="list" aria-label={m['search.resultsCaption']()} class="divide-y divide-border">
				{#each store.result.items as machine (`${machine.cluster}:${machine.vmid}`)}
					<li data-testid="search-result-row">
						<a
							href={vmHref(machine)}
							class="group block p-4 transition-colors hover:bg-muted/40 focus-visible:bg-muted/60 focus-visible:outline-none"
							data-testid="search-result-link"
						>
							<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
								<div class="min-w-0 flex-1">
									<p class="truncate text-base font-medium text-foreground group-hover:text-primary">
										{machine.name}
									</p>
									{#if machine.tags.length > 0}
										<div class="mt-1.5 flex flex-wrap gap-1.5">
											{#each machine.tags as tag (tag)}
												<span
													class="rounded-full bg-muted px-2 py-0.5 text-xs text-muted-foreground"
													data-testid="search-result-tag"
												>
													{tag}
												</span>
											{/each}
										</div>
									{/if}
								</div>
								<div class="shrink-0">
									<Pill tone={statusTone[machine.status]} label={statusLabels[machine.status]()} />
								</div>
							</div>

							<dl class="mt-3 grid grid-cols-2 gap-x-4 gap-y-1 sm:grid-cols-3">
								<div>
									<dt class="text-xs text-muted-foreground">{m['vms.list.columnCluster']()}</dt>
									<dd class="font-mono text-sm text-foreground">{machine.clusterDisplayName}</dd>
								</div>
								<div>
									<dt class="text-xs text-muted-foreground">{m['vms.list.columnNode']()}</dt>
									<dd class="font-mono text-sm text-foreground">{machine.node}</dd>
								</div>
								<div>
									<dt class="text-xs text-muted-foreground">{m['vms.list.columnId']()}</dt>
									<dd class="font-mono text-sm text-foreground">{machine.vmid}</dd>
								</div>
							</dl>
						</a>
					</li>
				{/each}
			</ul>
		</Card>
	{/if}
</section>
