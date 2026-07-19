<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Plus } from 'phosphor-svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import type { VMCreateSettings } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';
	import DiskCard from '../_components/DiskCard.svelte';

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
	}

	let { store, settings }: Props = $props();
</script>

<div class="flex items-center justify-between">
	<h2 class="text-lg font-semibold">{$t('vmCreate.disk.title')}</h2>
	<Badge variant="secondary">
		{$t('vmCreate.disk.diskCount', {
			values: { current: String(store.form.disks.length), max: String(settings.maxDiskPerVm) }
		})}
	</Badge>
</div>
<p class="text-muted-foreground text-xs">
	{$t('vmCreate.disk.capacityInfo', {
		values: {
			total: String(store.totalDiskGB),
			max: String(store.maxTotalDiskGB),
			maxDisks: String(settings.maxDiskPerVm),
			maxSize: String(settings.limits.disk.max)
		}
	})}
</p>
<Separator />
<div class="space-y-4">
	{#each store.form.disks as _disk, i (i)}
		<DiskCard {store} {settings} index={i} />
	{/each}
	{#if store.form.disks.length < settings.maxDiskPerVm}
		<Button variant="outline" size="sm" onclick={() => store.addDisk()}>
			<Plus class="mr-1 h-4 w-4" />
			{$t('vmCreate.disk.addDisk')}
		</Button>
	{:else}
		<p class="text-muted-foreground text-xs">
			{$t('vmCreate.disk.maxDisksReached', {
				values: { max: String(settings.maxDiskPerVm) }
			})}
		</p>
	{/if}
</div>
