<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setVmListContext } from '$lib/features/vms/list.svelte';
	import { setVmBulkContext } from '$lib/features/vms/bulk.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import MachineList from '$lib/features/vms/MachineList.svelte';
	import VmBulkActionBar from '$lib/features/vms/VmBulkActionBar.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import { fetchClusterOptions, type ClusterOption } from '$lib/shared/clusters';
	import { m } from '$lib/paraglide/messages.js';

	// Wiring only: the list state, URL sync, and rendering all live in
	// $lib/features/vms (FR-010) - this page just picks the scope.
	let clusterOptions = $state<ClusterOption[]>([]);
	const session = getSessionContext();

	const vmListStore = setVmListContext({
		scope: 'mine',
		initialQuery: page.url.search,
		navigate: (queryString: string) => {
			// resolve() only accepts route literals, so the query string is
			// appended after the typed route resolution - no `as '/vms'` cast.
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
		vmListStore.dispose();
		offTaskOk?.();
	});
</script>

<svelte:head>
	<title>{m['vms.list.title']()}</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl">
	<PageHeader
		eyebrow={m['vms.list.eyebrow']()}
		title={m['vms.list.calmHeading']()}
		description={m['vms.list.calmDescription']()}
		focusTarget
		divider={false}
	>
		{#snippet actions()}
			<div class="flex flex-wrap items-center gap-2">
				{#if clusterOptions.length > 1}
					<ClusterSelector options={clusterOptions} value={vmListStore.cluster} onChange={(value) => vmListStore.setCluster(value)} includeAll id="vm-cluster-filter" />
				{/if}
				<Button
					variant="secondary"
					size="md"
					disabled={vmListStore.loading}
					onclick={() => void vmListStore.load()}
				>
					{vmListStore.loading ? m['common.refreshing']() : m['common.refresh']()}
				</Button>
				{#if !session.isAdmin}
					<ButtonLink href={resolve('/vms/create')} data-testid="vm-create-link">
						<svg viewBox="0 0 24 24" class="h-4 w-4" fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" aria-hidden="true">
							<line x1="12" y1="5" x2="12" y2="19" />
							<line x1="5" y1="12" x2="19" y2="12" />
						</svg>
						{m['vms.list.createAction']()}
					</ButtonLink>
				{/if}
			</div>
		{/snippet}
	</PageHeader>

	<VmBulkActionBar />
	<MachineList />
</section>
