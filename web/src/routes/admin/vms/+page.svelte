<script lang="ts">
	/**
	 * Admin machine collection - every PVMSS VM of every pool, across every
	 * cluster. Wiring only: the list state (search, filters, sort, page) is
	 * the shared VmListStore instance scoped to 'all' (admin-only server
	 * side), and the rendering lives in #lib/features/admin-vms.
	 */
	import { onDestroy, onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setVmListContext } from '#lib/features/vms/list.svelte.js';
	import { getTaskTrayContext } from '#lib/features/tasks/tasks.svelte.js';
	import AdminVmsPage from '#lib/features/admin-vms/AdminVmsPage.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import { fetchClusterOptions, type ClusterOption } from '#lib/shared/clusters.js';
	import { post } from '#lib/shared/api/client.js';
	import { onVisibleRefresh } from '#lib/shared/visibility-refresh.js';
	import { m } from '#lib/paraglide/messages.js';

	let clusterOptions = $state<ClusterOption[]>([]);
	let refreshing = $state(false);

	const store = setVmListContext({
		scope: 'all',
		initialQuery: page.url.search,
		navigate: (queryString: string) => {
			const base = resolve('/admin/vms');
			const target = queryString === '' ? base : `${base}?${queryString}`;
			void goto(target, { replace: true, reset: false });
		}
	});

	let offVisible: (() => void) | null = null;
	let offTaskOk: (() => void) | null = null;

	async function loadPage(): Promise<void> {
		try {
			clusterOptions = await fetchClusterOptions();
		} catch {
			clusterOptions = [];
		}
		await store.load();
	}

	// The list reads the cached inventory, so force an inventory refresh first.
	// A throttled or failed refresh is ignored: the reload still shows current rows.
	async function refreshList(): Promise<void> {
		refreshing = true;
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// Throttled (429) or cluster unreachable: re-read what the inventory has.
		}
		try {
			await store.load();
		} finally {
			refreshing = false;
		}
	}

	onMount(() => {
		void loadPage();
		// A task finishing elsewhere (create, delete, power from the owner's
		// page) changes rows this collection shows too.
		offTaskOk = getTaskTrayContext().onTaskOk((task) => void store.loadAfterTask(task));
		offVisible = onVisibleRefresh(() => void store.refreshIfStale());
	});
	onDestroy(() => {
		store.dispose();
		offTaskOk?.();
		offVisible?.();
	});
</script>

<svelte:head>
	<title>{m['admin.vms.title']()}</title>
</svelte:head>

<section class="w-full">
	<PageHeader
		title={m['admin.vms.heading']()}
		eyebrow={m['chrome.adminNav.infrastructure']()}
		description={m['admin.vms.description']()}
	>
		{#snippet actions()}
			<div class="flex flex-wrap items-center gap-2">
				{#if clusterOptions.length > 1}
					<ClusterSelector
						options={clusterOptions}
						value={store.cluster}
						onChange={(value) => store.setCluster(value)}
						includeAll
						id="admin-vm-cluster-filter"
					/>
				{/if}
				<Button
					variant="secondary"
					size="md"
					disabled={store.loading || refreshing}
					onclick={() => void refreshList()}
					data-testid="admin-vm-refresh"
				>
					{store.loading || refreshing ? m['common.refreshing']() : m['common.refresh']()}
				</Button>
			</div>
		{/snippet}
	</PageHeader>

	<AdminVmsPage />
</section>
