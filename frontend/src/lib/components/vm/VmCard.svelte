<script lang="ts">
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import type { VMSummary } from '$lib/api/vms';
	import type { VMAction } from '$lib/types/vm';
	import VmStatusBadge from './VmStatusBadge.svelte';
	import VmActionButtons from './VmActionButtons.svelte';

	interface Props {
		vm: VMSummary;
		busy?: boolean;
		onAction?: (vm: VMSummary, action: VMAction) => void;
	}

	let { vm, busy = false, onAction }: Props = $props();

	function uptimeLabel(seconds: number | null | undefined): string {
		if (!seconds || seconds < 0) return '—';

		const yearSeconds = 31536000; // 365 days
		if (seconds > yearSeconds) {
			const years = Math.floor(seconds / yearSeconds);
			const remaining = seconds % yearSeconds;
			const days = Math.floor(remaining / 86400);
			return days > 0 ? `${years}y ${days}d` : `${years}y`;
		}

		const d = Math.floor(seconds / 86400);
		const h = Math.floor((seconds % 86400) / 3600);
		if (d > 0) return `${d}d ${h}h`;

		const m = Math.floor((seconds % 3600) / 60);
		return h > 0 ? `${h}h ${m}m` : `${m}m`;
	}
</script>

<tr
	class="pv-row pv-row--clickable"
	onclick={() => goto(`/vm/${vm.vmid}`)}
>
	<td class="pv-td-mono text-sm">{vm.vmid}</td>
	<td>
		<div class="pv-resource-cell">
			<div class="pv-resource-icon pv-resource-icon--vm text-[0.6rem]">VM</div>
			<span class="pv-resource-name">{vm.name || '—'}</span>
		</div>
	</td>
	<td>
		<VmStatusBadge status={vm.status} />
	</td>
	<td class="pv-td-muted text-sm">{vm.node || '—'}</td>
	<td class="pv-td-muted tabular-nums text-sm">{uptimeLabel(vm.uptime)}</td>
	{#if onAction}
		<td onclick={(e: MouseEvent) => e.stopPropagation()}>
			<VmActionButtons
				status={vm.status}
				{busy}
				onAction={(action) => onAction(vm, action)}
			/>
		</td>
	{/if}
</tr>
