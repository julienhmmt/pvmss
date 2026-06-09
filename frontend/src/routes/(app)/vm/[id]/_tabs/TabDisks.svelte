<script lang="ts">
	import { HardDrive } from 'phosphor-svelte';
	import { X } from 'phosphor-svelte';
	import { LinkBreak } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import { updateVMCDROM, disconnectVMCDROM } from '$lib/api/vm-details';
	import type { VMConfig, VMMetrics, DiskInfo, ISOOption } from '$lib/api/vm-details';
	import { t } from 'svelte-i18n';

	interface Props {
		config: VMConfig;
		metrics: VMMetrics | null;
		availableIsos: ISOOption[];
		onOpenAdd: () => void;
		onOpenResize: (disk: DiskInfo) => void;
		onOpenDelete: (disk: DiskInfo) => void;
		onRefresh: () => void;
	}

	let { config, metrics, availableIsos, onOpenAdd, onOpenResize, onOpenDelete, onRefresh }: Props = $props();

	const isRunning = $derived((metrics?.status ?? config.status) === 'running');

	let cdromEditing = $state(false);
	let selectedIso = $state('');
	let cdromLoading = $state(false);

	function startEdit() {
		selectedIso = config.currentIso || '';
		cdromEditing = true;
	}

	// Reset selectedIso when canceling edit
	function cancelEdit() {
		selectedIso = config.currentIso || '';
		cdromEditing = false;
	}

	async function saveCDROM() {
		cdromLoading = true;
		try {
			await updateVMCDROM(config.vmid, selectedIso);
			toast.success($t('vm.cdromUpdated'));
			cdromEditing = false;
			onRefresh();
		} catch {
			toast.error($t('common.error'));
		} finally {
			cdromLoading = false;
		}
	}

	async function removeCDROM() {
		cdromLoading = true;
		try {
			await updateVMCDROM(config.vmid, '');
			toast.success($t('vm.cdromRemoved'));
			cdromEditing = false;
			onRefresh();
		} catch {
			toast.error($t('common.error'));
		} finally {
			cdromLoading = false;
		}
	}

	async function disconnectCDROM() {
		cdromLoading = true;
		try {
			await disconnectVMCDROM(config.vmid);
			toast.success($t('vm.cdromDisconnected'));
			cdromEditing = false;
			onRefresh();
		} catch {
			toast.error($t('common.error'));
		} finally {
			cdromLoading = false;
		}
	}
</script>

<div class="pv-table-wrap">
	<div class="flex items-center justify-between border-b border-border px-4 py-3">
		<span class="text-sm font-medium">{$t('vm.tabDisks')}</span>
		<button
			class="inline-flex items-center gap-1 rounded-md bg-primary px-3 py-1.5 text-xs font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50 disabled:cursor-not-allowed"
			onclick={onOpenAdd}
			disabled={isRunning}
			title={isRunning ? $t('vm.disk.vmRunningWarning') : ''}
		>
			+ {$t('vm.disk.add')}
		</button>
	</div>
	{#if config.disks.length === 0}
		<p class="py-8 text-center text-sm text-muted-foreground">{$t('vm.noDisks')}</p>
	{:else}
		<table class="pv-table">
			<thead>
				<tr>
					<th>{$t('common.name')}</th>
					<th>{$t('common.storage')}</th>
					<th>{$t('common.size')}</th>
					<th>{$t('common.actions')}</th>
				</tr>
			</thead>
			<tbody>
				{#each config.disks as disk (disk.index)}
					<tr class="pv-row">
						<td>
							<div class="flex items-center gap-2">
								<HardDrive class="h-4 w-4 text-muted-foreground" />
								<span class="text-sm">{disk.index}</span>
							</div>
						</td>
						<td class="text-sm">{disk.storage}</td>
						<td class="text-sm">{disk.sizeGb} GB</td>
						<td>
							<div class="flex gap-2">
								<button
									class="text-xs text-primary hover:underline"
									onclick={() => onOpenResize(disk)}
									disabled={isRunning}
								>
									{$t('vm.disk.resize')}
								</button>
								<button
									class="text-xs text-destructive hover:underline"
									onclick={() => onOpenDelete(disk)}
									disabled={isRunning}
								>
									{$t('vm.disk.detach')}
								</button>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
	<div class="border-t border-border px-4 py-3">
		<div class="mb-2 flex items-center gap-2">
			<HardDrive class="h-4 w-4 text-muted-foreground" />
			<span class="text-sm font-medium">CD-ROM</span>
			{#if !cdromEditing}
				<button class="ml-auto rounded border border-border bg-background px-2 py-1 text-xs hover:bg-accent" onclick={startEdit}>
					{$t('common.edit')}
				</button>
			{/if}
		</div>
		{#if cdromEditing}
			<div class="flex flex-col gap-2">
				{#if config.currentIso}
					<div class="text-xs text-muted-foreground">{$t('vm.currentISO')}: {config.currentIso}</div>
				{/if}
				<select class="rounded border border-border bg-background px-3 py-2 text-sm" bind:value={selectedIso}>
					<option value="">-- {$t('vm.noISO')} --</option>
					{#each availableIsos as iso (iso.volid)}
						<option value={iso.volid}>{iso.name}</option>
					{/each}
					{#if config.currentIso && !availableIsos.find(iso => iso.volid === config.currentIso)}
						<option value={config.currentIso}>{config.currentIso}</option>
					{/if}
				</select>
				<div class="flex flex-wrap gap-2">
					<button class="pv-btn-primary text-xs" onclick={saveCDROM} disabled={cdromLoading}>
						{cdromLoading ? $t('common.saving') : $t('common.save')}
					</button>
					{#if config.currentIso}
						<button class="rounded border border-border bg-background px-2 py-1 text-xs hover:bg-accent" onclick={disconnectCDROM} disabled={cdromLoading}>
							<LinkBreak class="inline h-3 w-3" />
							{$t('vm.disconnect')}
						</button>
					{/if}
					{#if config.hasCdrom}
						<button class="rounded border border-destructive bg-background px-2 py-1 text-xs text-destructive hover:bg-destructive/10" onclick={removeCDROM} disabled={cdromLoading}>
							<X class="inline h-3 w-3" />
							{$t('common.clear')}
						</button>
					{/if}
					<button class="text-xs text-muted-foreground hover:text-foreground" onclick={cancelEdit}>
						{$t('common.cancel')}
					</button>
				</div>
			</div>
		{:else}
			<div class="text-sm text-muted-foreground">{config.currentIso || $t('vm.noISO')}</div>
		{/if}
	</div>
</div>
