<script lang="ts">
	/**
	 * MachineRow - one row of the machine collection (extracted from
	 * MachineList, ui-review-remediation/05): identity (OS mark, name,
	 * subtitle), resources, status pill and one labelled action, with a
	 * muted hint line when the state needs explaining. The name link is
	 * stretched over the whole row (after:inset-0), so a click anywhere on
	 * it opens the machine; the checkbox and the action sit above the
	 * overlay (relative z-10).
	 */
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import { getVmBulkContext } from './bulk.svelte';
	import type { VmListItem as ListItem } from './list.svelte';
	import { compactBytes, rowAction, rowHint } from './machine-row';
	import type { MachineDisplayStatus } from './display-status';
	import MachineStatusPill from './MachineStatusPill.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import ButtonLink from '#lib/shared/ui/ButtonLink.svelte';
	import OsMark from '#lib/shared/ui/OsMark.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';

	interface Props {
		machine: ListItem;
		status: MachineDisplayStatus;
		busy: boolean;
		selectMode: boolean;
		/** The cluster chip only tells rows apart when the list mixes clusters. */
		spansClusters: boolean;
		onStart: (machine: ListItem) => void;
	}

	let { machine, status, busy, selectMode, spansClusters, onStart }: Props = $props();

	const bulk = getVmBulkContext();

	const hint = $derived(rowHint(status));
	const action = $derived(rowAction(status));

	function detailHref(target: ListItem): string {
		return resolve('/vms/[cluster]/[vmid]', { cluster: target.cluster, vmid: String(target.vmid) });
	}

	function consoleHref(target: ListItem): string {
		return resolve('/vms/[cluster]/[vmid]/console', { cluster: target.cluster, vmid: String(target.vmid) });
	}

	/** Tags minus the internal marker; the cluster no longer rides here - it
	 *  is either the chip beside the name or implied by the active filter. */
	function visibleTags(target: ListItem): string[] {
		return target.tags.filter((tag) => tag !== 'pvmss');
	}
</script>

<li class="relative border-b border-border-subtle px-4 py-3.5 last:border-b-0 hover:bg-muted/40" data-testid="vm-row" data-status={status}>
	<div class="flex items-center gap-3">
		{#if selectMode}
			<input
				type="checkbox"
				class="relative z-10 h-4 w-4 shrink-0 rounded border-border accent-primary"
				checked={bulk.isSelected(machine.cluster, machine.vmid)}
				onchange={() => bulk.toggle({ cluster: machine.cluster, vmid: machine.vmid })}
				data-testid="vm-bulk-select-row"
				aria-label={m['vms.list.selectRow']({ name: machine.name })}
			/>
		{/if}
		<div class="grid min-w-0 flex-1 grid-cols-[minmax(0,1fr)_auto] items-center gap-x-4 gap-y-2 min-[700px]:grid-cols-[minmax(0,1fr)_11rem_8.5rem_9rem]">
			<div class="flex min-w-0 items-center gap-3">
				<span class="max-[369px]:hidden"><OsMark ostype={machine.ostype} /></span>
				<div class="min-w-0 flex-1">
					<div class="flex min-w-0 items-center gap-2">
						<a
							href={detailHref(machine)}
							class="pv-focus block max-w-full shrink-0 truncate font-medium text-foreground underline-offset-2 after:absolute after:inset-0 after:content-[''] hover:text-primary hover:underline"
							aria-label={spansClusters
								? m['vms.list.rowLinkLabel']({ name: machine.name, cluster: machine.clusterDisplayName })
								: undefined}
							data-testid="vm-row-link"
						>
							{machine.name}
						</a>
						{#if spansClusters}
							<!-- The chip yields before the name does. -->
							<span class="min-w-0 shrink overflow-hidden" data-testid="vm-row-cluster">
								<Pill tone="off" dot={false} label={machine.clusterDisplayName} />
							</span>
						{/if}
					</div>
					{#if visibleTags(machine).length > 0}
						<p class="truncate text-xs text-muted-foreground">
							{visibleTags(machine).join(' · ')}
						</p>
					{/if}
					<!-- The wide resources column hides below 700px; this compact
					     line keeps size on the row where the list has to answer
					     "which machine is it / is it big enough" on a phone. -->
					<p class="mt-0.5 font-mono text-xs tabular-nums text-muted-foreground min-[700px]:hidden">
						{m['vms.list.resourcesCompute']({ cpu: machine.cpuCores, memory: compactBytes(machine.memoryTotal) })}
					</p>
				</div>
			</div>
			<p class="text-xs leading-5 text-muted-foreground max-[699px]:hidden">
				<span class="whitespace-nowrap font-mono tabular-nums">{m['vms.list.resourcesCompute']({ cpu: machine.cpuCores, memory: compactBytes(machine.memoryTotal) })}</span>
				<span class="block font-mono tabular-nums text-muted-foreground-subtle">VM {machine.vmid}</span>
			</p>
			<div class="max-[699px]:order-3">
				<MachineStatusPill {status} pending={busy || status === 'provisioning'} />
			</div>
			<div class="relative z-10 flex justify-end gap-2 max-[699px]:order-4 max-[699px]:col-span-2 max-[699px]:justify-start max-[699px]:pl-[50px] max-[369px]:pl-0">
				{#if status === 'running' && !busy}
					<ButtonLink
						variant="secondary"
						size="sm"
						href={consoleHref(machine)}
						target="_blank"
						rel="noopener noreferrer"
						aria-label={m['vms.list.actionConsoleFor']({ name: machine.name })}
						data-testid="vm-row-console"
					>
						{m['vms.list.actionConsole']()}
					</ButtonLink>
				{/if}
				{#if action === 'start'}
					<Button
						variant="secondary"
						size="sm"
						loading={busy}
						aria-label={m['vms.list.actionStartFor']({ name: machine.name })}
						onclick={() => onStart(machine)}
						data-testid="vm-row-start"
					>
						{m['vms.list.actionStart']()}
					</Button>
				{:else}
					<ButtonLink
						variant="secondary"
						size="sm"
						href={detailHref(machine)}
						aria-label={m['vms.list.actionDetailsFor']({ name: machine.name })}
						data-testid="vm-row-details"
					>
						{m['vms.list.actionDetails']()}
					</ButtonLink>
				{/if}
			</div>
		</div>
	</div>
	{#if hint}
		<p
			class="mt-1.5 text-xs {hint.tone === 'error' ? 'text-destructive' : 'text-muted-foreground'} {selectMode ? 'pl-7' : ''} min-[370px]:pl-[50px]"
			data-testid="vm-row-hint"
		>
			{hint.text}
		</p>
	{/if}
</li>
