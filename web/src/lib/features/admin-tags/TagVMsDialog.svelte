<script lang="ts">
	import { resolve } from '$app/paths';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { fetchTagVMs, type TagVM } from './tag-vms';

	interface Props {
		/** Tag whose VMs are listed; null keeps the dialog closed. */
		tag: string | null;
		cluster: string;
		onClose: () => void;
	}

	let { tag, cluster, onClose }: Props = $props();

	let vms = $state<TagVM[]>([]);
	let loading = $state(false);
	let error = $state<string | null>(null);

	$effect(() => {
		const name = tag;
		if (name === null) return;
		let cancelled = false;
		vms = [];
		error = null;
		loading = true;
		fetchTagVMs(cluster, name)
			.then((result) => {
				if (!cancelled) vms = result;
			})
			.catch(() => {
				if (!cancelled) error = m['admin.tags.vmsError']();
			})
			.finally(() => {
				if (!cancelled) loading = false;
			});
		return () => {
			cancelled = true;
		};
	});
</script>

<Dialog open={tag !== null} size="lg" labelledBy="tag-vms-title" {onClose}>
	<h2 id="tag-vms-title" class="mb-4 text-lg font-medium">
		{m['admin.tags.vmsDialogTitle']({ name: tag ?? '' })}
	</h2>
	{#if loading}
		<div role="status" aria-live="polite" class="sr-only">{m['admin.tags.vmsLoading']()}</div>
		<TableSkeleton columns={4} />
	{:else if error}
		<Alert>{error}</Alert>
	{:else if vms.length === 0}
		<EmptyState title={m['admin.tags.vmsEmpty']()} />
	{:else}
		<table class="pv-table pv-responsive-table">
			<thead>
				<tr>
					<th class="font-medium">{m['common.name']()}</th>
					<th class="font-medium">{m['admin.tags.vmsColumnVmid']()}</th>
					<th class="font-medium">{m['admin.tags.vmsColumnLocation']()}</th>
					<th class="font-medium">{m['common.status']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each vms as vm (`${vm.cluster}/${vm.vmid}`)}
					<tr>
						<td data-label={m['common.name']()}>
							<a
								href={resolve('/vms/[cluster]/[vmid]', { cluster: vm.cluster, vmid: String(vm.vmid) })}
								class="pv-focus font-medium text-foreground underline-offset-2 hover:text-primary hover:underline"
							>{vm.name}</a>
						</td>
						<td data-label={m['admin.tags.vmsColumnVmid']()}>{vm.vmid}</td>
						<td data-label={m['admin.tags.vmsColumnLocation']()}>{vm.cluster} / {vm.node}</td>
						<td data-label={m['common.status']()} class="font-mono text-xs">{vm.status}</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</Dialog>
