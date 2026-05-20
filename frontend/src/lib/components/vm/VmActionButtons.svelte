<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Play, Stop, ArrowCounterClockwise } from 'phosphor-svelte';
	import type { VMAction, VMStatus } from '$lib/types/vm';

	interface Props {
		status: VMStatus;
		busy?: boolean;
		onAction: (action: VMAction) => void;
	}

	let { status, busy = false, onAction }: Props = $props();

	function canStart(s: VMStatus): boolean {
		return s === 'stopped' || s === 'paused';
	}

	function canStop(s: VMStatus): boolean {
		return s === 'running';
	}
</script>

<div class="flex items-center gap-1">
	{#if canStart(status)}
		<button
			class="pv-action-btn pv-action-btn--start"
			onclick={() => onAction('start')}
			disabled={busy}
			title={$t('vms.actions.start')}
		>
			<Play class="h-3.5 w-3.5" weight="fill" />
		</button>
	{:else if canStop(status)}
		<button
			class="pv-action-btn pv-action-btn--stop"
			onclick={() => onAction('shutdown')}
			disabled={busy}
			title={$t('vms.actions.shutdown')}
		>
			<Stop class="h-3.5 w-3.5" weight="fill" />
		</button>
		<button
			class="pv-action-btn"
			onclick={() => onAction('reboot')}
			disabled={busy}
			title={$t('vms.actions.reboot')}
		>
			<ArrowCounterClockwise class="h-3.5 w-3.5" />
		</button>
	{/if}
</div>
