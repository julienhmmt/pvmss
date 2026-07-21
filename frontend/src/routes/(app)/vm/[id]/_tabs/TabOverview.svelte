<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import {
		CopySimple,
		Cpu,
		Desktop,
		HardDrive,
		PencilSimple,
		ShieldCheck,
		Shield,
		Disc
	} from 'phosphor-svelte';
	import type { VMConfig, VMMetrics } from '$lib/api/vm-details';

	interface Props {
		config: VMConfig;
		metrics: VMMetrics | null;
		onOpenHardware: () => void;
	}

	let { config, metrics, onOpenHardware }: Props = $props();

	const currentStatus = $derived(metrics?.status ?? config.status);
	const totalStorageGb = $derived(config.disks.reduce((s, d) => s + d.sizeGb, 0));
	const nicCount = $derived(config.networks.length);

	async function copy(text: string, label: string): Promise<void> {
		try {
			await navigator.clipboard.writeText(text);
			toast.success($t('common.copied', { values: { value: label } }));
		} catch {
			toast.error($t('common.copyFailed'));
		}
	}
</script>

<div class="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
	<!-- System -->
	<div class="pv-card">
		<div class="pv-card-header flex items-center gap-2">
			<Desktop class="h-4 w-4" />
			{$t('vm.sectionOverview')}
		</div>
		<div class="pv-card-body space-y-2.5 text-sm">
			<div class="flex items-center justify-between">
				<span class="text-muted-foreground">{$t('common.vmid')}</span>
				<span class="pv-copy-cell font-mono">
					{config.vmid}
					<button class="pv-copy-btn" onclick={() => copy(String(config.vmid), $t('common.vmid'))} title={$t('common.copy')}>
						<CopySimple class="h-3 w-3" />
					</button>
				</span>
			</div>
			<div class="flex items-center justify-between">
				<span class="text-muted-foreground">{$t('common.node')}</span>
				<span class="font-medium">{config.node}</span>
			</div>
			<div class="flex items-center justify-between">
				<span class="text-muted-foreground">{$t('common.status')}</span>
				<span class="font-medium capitalize">{currentStatus}</span>
			</div>
			{#if config.uptime > 0}
				<div class="flex items-center justify-between">
					<span class="text-muted-foreground">{$t('vm.uptime')}</span>
					<span class="font-mono text-xs">
						{Math.floor(config.uptime / 3600)}h {Math.floor((config.uptime % 3600) / 60)}m
					</span>
				</div>
			{/if}
			<div class="flex items-start justify-between gap-2">
				<span class="text-muted-foreground">{$t('common.tags')}</span>
				<div class="flex flex-wrap gap-1 text-right">
					{#if config.tags}
						{#each config.tags.split(';').filter(Boolean) as tag (tag)}
							<span class="rounded bg-muted px-1.5 py-px text-xs">{tag}</span>
						{/each}
					{:else}
						<span class="text-muted-foreground">—</span>
					{/if}
				</div>
			</div>
		</div>
	</div>

	<!-- Hardware -->
	<div class="pv-card">
		<div class="pv-card-header flex items-center justify-between">
			<div class="flex items-center gap-2">
				<Cpu class="h-4 w-4" />
				{$t('vm.sectionHardware')}
			</div>
			<Button size="sm" variant="ghost" onclick={onOpenHardware} class="h-7 px-2 text-xs">
				<PencilSimple class="mr-1 h-3.5 w-3.5" />
				{$t('common.edit')}
			</Button>
		</div>
		<div class="pv-card-body space-y-2.5 text-sm">
			<div class="flex items-center justify-between">
				<span class="text-muted-foreground">{$t('common.cpu')}</span>
				<span>
					{config.sockets} × {config.cores} = <span class="font-semibold">{config.cpus}</span> {$t('common.vcpuCount', { values: { count: config.cpus } })}
				</span>
			</div>
			<div class="flex items-center justify-between">
				<span class="text-muted-foreground">{$t('vms.ram')}</span>
				<span class="font-medium">{Math.round(config.maxMemMb / 1024)} {$t('common.gib')}</span>
			</div>
			<div class="flex items-center justify-between">
				<span class="text-muted-foreground">{$t('common.storage')}</span>
				<span class="font-medium">{totalStorageGb} {$t('common.gb')} <span class="text-xs text-muted-foreground">({config.disks.length} {$t('vm.diskCount', { values: { count: config.disks.length } })})</span></span>
			</div>
			<div class="flex items-center justify-between">
				<span class="text-muted-foreground">{$t('vm.interface')}</span>
				<span class="font-medium">{nicCount} {$t('vm.interfaceCount', { values: { count: nicCount } })}</span>
			</div>
		</div>
	</div>

	<!-- Features -->
	<div class="pv-card">
		<div class="pv-card-header">{$t('vm.sectionFeatures')}</div>
		<div class="pv-card-body grid grid-cols-2 gap-x-4 gap-y-3 text-sm">
			<div class="flex items-center gap-2">
				{#if config.efiEnabled}
					<ShieldCheck class="h-4 w-4 text-success" />
					<span class="font-medium">{$t('common.efiUefi')}</span>
				{:else}
					<Shield class="h-4 w-4 text-muted-foreground" />
					<span class="text-muted-foreground">{$t('common.legacyBios')}</span>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				{#if config.tpmEnabled}
					<ShieldCheck class="h-4 w-4 text-success" />
					<span class="font-medium">{$t('common.tpm')}</span>
				{:else}
					<Shield class="h-4 w-4 text-muted-foreground" />
					<span class="text-muted-foreground">{$t('common.noTpm')}</span>
				{/if}
			</div>
			<div class="flex items-center gap-2">
				<Disc class="h-4 w-4 {config.hasCdrom ? 'text-foreground' : 'text-muted-foreground'}" />
				<span class={config.hasCdrom ? 'font-medium' : 'text-muted-foreground'}>
					{config.hasCdrom ? $t('vm.cdromAttached') : $t('vm.noCdrom')}
				</span>
			</div>
			<div class="flex items-center gap-2">
				<HardDrive class="h-4 w-4 {config.currentIso ? 'text-foreground' : 'text-muted-foreground'}" />
				<span class="truncate {config.currentIso ? 'font-medium' : 'text-muted-foreground'}">
					{config.currentIso || $t('vm.noISO')}
				</span>
			</div>
			<div class="col-span-2 pt-1 text-xs text-muted-foreground">
				{config.description || $t('vm.noDescription')}
			</div>
		</div>
	</div>
</div>
