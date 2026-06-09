<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Play, Stop, ArrowCounterClockwise, ArrowsClockwise, Monitor, Trash } from 'phosphor-svelte';

	interface Props {
		vmid: number;
		name: string | null;
		node: string | null;
		status: string;
		actionLoading: boolean;
		onAction: (action: string) => void;
		onRefresh: () => void;
		onConsole: () => void;
		onDelete: () => void;
	}

	let {
		vmid,
		name,
		node,
		status,
		actionLoading,
		onAction,
		onRefresh,
		onConsole,
		onDelete
	}: Props = $props();

	const isRunning = $derived(status === 'running');
</script>

<div class="inline-flex items-center gap-1 rounded-lg border border-border bg-card p-1 shadow-sm">
	<button
		class="pv-action-btn"
		onclick={onRefresh}
		disabled={actionLoading}
		title={$t('common.refresh')}
	>
		<ArrowsClockwise class="h-4 w-4" />
	</button>
	{#if !isRunning}
		<button
			class="pv-action-btn pv-action-btn--start"
			onclick={() => onAction('start')}
			disabled={actionLoading}
			title={$t('vms.actions.start')}
		>
			<Play class="h-4 w-4" weight="fill" />
		</button>
	{:else}
		<button
			class="pv-action-btn pv-action-btn--stop"
			onclick={() => onAction('shutdown')}
			disabled={actionLoading}
			title={$t('vms.actions.shutdown')}
		>
			<Stop class="h-4 w-4" weight="fill" />
		</button>
		<button
			class="pv-action-btn pv-action-btn--halt"
			onclick={() => onAction('stop')}
			disabled={actionLoading}
			title={$t('vms.actions.forceStop')}
		>
			<Stop class="h-4 w-4" />
		</button>
		<button
			class="pv-action-btn"
			onclick={() => onAction('reboot')}
			disabled={actionLoading}
			title={$t('vms.actions.reboot')}
		>
			<ArrowCounterClockwise class="h-4 w-4" />
		</button>
	{/if}
	{#if isRunning}
		<button
			class="pv-action-btn"
			onclick={onConsole}
			title={$t('vm.openConsole')}
		>
			<Monitor class="h-4 w-4" />
		</button>
	{/if}
	<button
		class="pv-action-btn pv-action-btn--stop"
		onclick={onDelete}
		disabled={actionLoading}
		title={$t('vm.delete')}
	>
		<Trash class="h-4 w-4" />
	</button>
</div>
