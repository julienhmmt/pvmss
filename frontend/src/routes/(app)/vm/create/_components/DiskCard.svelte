<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Trash } from 'phosphor-svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import type { VMCreateSettings } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
		index: number;
	}

	let { store, settings, index }: Props = $props();
</script>

<div class="bg-muted/50 flex items-center gap-3 rounded-lg p-3">
	<div class="flex-1 space-y-1">
		<Label>
			{index === 0
				? $t('vmCreate.disk.primaryDisk')
				: $t('vmCreate.disk.diskIndex', { values: { index: String(index + 1) } })}
		</Label>
		<div class="flex items-center gap-2">
			<Input
				type="number"
				bind:value={store.form.disks[index].sizeGb}
				min={settings.limits.disk.min}
				max={settings.limits.disk.max}
				class="w-32"
			/>
			<span class="text-muted-foreground text-sm">GB</span>
			<span class="text-muted-foreground text-xs">
				({settings.limits.disk.min} – {settings.limits.disk.max})
			</span>
		</div>
	</div>
	{#if index > 0}
		<Button
			variant="ghost"
			size="sm"
			aria-label={$t('common.remove')}
			onclick={() => store.removeDisk(index)}
			class="text-destructive"
		>
			<Trash class="h-4 w-4" />
		</Button>
	{/if}
</div>
