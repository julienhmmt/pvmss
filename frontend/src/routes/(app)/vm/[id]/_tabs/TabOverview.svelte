<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { Button } from '$lib/components/ui/button';
	import { CopySimple } from 'phosphor-svelte';
	import type { VMConfig, VMMetrics } from '$lib/api/vm-details';

	interface Props {
		config: VMConfig;
		metrics: VMMetrics | null;
		onOpenHardware: () => void;
	}

	let { config, metrics, onOpenHardware }: Props = $props();

	const currentStatus = $derived(metrics?.status ?? config.status);

	async function copyVmid(): Promise<void> {
		try {
			await navigator.clipboard.writeText(String(config.vmid));
			toast.success($t('common.copied', { values: { value: 'VMID' } }));
		} catch {
			toast.error($t('common.copyFailed'));
		}
	}
</script>

<div class="grid gap-4 md:grid-cols-2">
	<div class="pv-card">
		<div class="pv-card-header">{$t('vm.sectionOverview')}</div>
		<div class="pv-card-body space-y-2 text-sm">
			<p>
				<span class="text-muted-foreground">VMID:</span>
				<span class="pv-copy-cell ml-1">
					<span class="font-mono">{config.vmid}</span>
					<button
						class="pv-copy-btn"
						onclick={copyVmid}
						title={$t('common.copy')}
						aria-label={$t('common.copy')}
					>
						<CopySimple class="h-3 w-3" />
					</button>
				</span>
			</p>
			<p><span class="text-muted-foreground">Node:</span> {config.node}</p>
			<p><span class="text-muted-foreground">Status:</span> {currentStatus}</p>
			<p><span class="text-muted-foreground">Tags:</span> {config.tags || '—'}</p>
		</div>
	</div>
	<div class="pv-card">
		<div class="pv-card-header">{$t('vm.sectionHardware')}</div>
		<div class="pv-card-body space-y-2 text-sm">
			<p><span class="text-muted-foreground">CPU:</span> {config.cpus} vCPU</p>
			<p><span class="text-muted-foreground">Memory:</span> {Math.round(config.maxMemMb / 1024)} GiB</p>
			<p><span class="text-muted-foreground">Disks:</span> {config.disks.length}</p>
			<Button size="sm" variant="outline" onclick={onOpenHardware}>{$t('common.edit')}</Button>
		</div>
	</div>
</div>


