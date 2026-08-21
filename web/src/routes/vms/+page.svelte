<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setVmListContext } from '$lib/features/vms/list.svelte';
	import { setVmBulkContext } from '$lib/features/vms/bulk.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import VmList from '$lib/features/vms/VmList.svelte';
	import VmBulkActionBar from '$lib/features/vms/VmBulkActionBar.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
	import { m } from '$lib/paraglide/messages.js';

	// Wiring only: the list state, URL sync, and rendering all live in
	// $lib/features/vms (FR-010) — this page just picks the scope.
	let clusterOptions = $state<ClusterOption[]>([]);
	const session = getSessionContext();

	const vmListStore = setVmListContext({
		scope: 'mine',
		initialQuery: page.url.search,
		navigate: (queryString: string) => {
			// resolve() only accepts route literals, so the query string is
			// appended after the typed route resolution — no `as '/vms'` cast.
			const base = resolve('/vms');
			const target = queryString === '' ? base : `${base}?${queryString}`;
			void goto(target, {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});
	const vmBulk = setVmBulkContext();

	let offTaskOk: (() => void) | null = null;

	async function loadPage(): Promise<void> {
		try {
			clusterOptions = await fetchClusterOptions();
		} catch {
			clusterOptions = [];
		}
		await vmListStore.load();
	}

	onMount(() => {
		void loadPage();
		offTaskOk = getTaskTrayContext().onTaskOk(() => {
			void vmListStore.load();
			vmBulk.clearResult();
		});
	});
	onDestroy(() => {
		offTaskOk?.();
	});
</script>

<svelte:head>
	<title>{m['vms.list.title']()}</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl py-2">
	<PageHeader title={m['vms.list.heading']()} description={m['vms.list.description']()}>
		{#snippet actions()}
			<div class="flex flex-wrap items-center gap-2">
				<ClusterSelector options={clusterOptions} value={vmListStore.cluster} onChange={(value) => vmListStore.setCluster(value)} includeAll id="vm-cluster-filter" />
				<Button
					variant="secondary"
					size="sm"
					disabled={vmListStore.loading}
					onclick={() => void vmListStore.load()}
				>
					{vmListStore.loading ? m['common.refreshing']() : m['common.refresh']()}
				</Button>
				{#if !session.isAdmin}
					<a
						href={resolve('/vms/create')}
						class="inline-flex items-center justify-center rounded-[0.625rem] bg-primary px-4 py-2.5 text-sm font-semibold text-primary-foreground shadow-card transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background"
					>
						{m['vms.list.create']()}
					</a>
				{/if}
			</div>
		{/snippet}
	</PageHeader>

	{#if vmListStore.loading && vmListStore.result === null}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<TableSkeleton columns={9} />
	{:else}
		<div class="fade-in">
			<VmBulkActionBar />
			<VmList />
		</div>
	{/if}
</section>
