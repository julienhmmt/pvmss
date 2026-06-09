<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Badge } from '$lib/components/ui/badge';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import { Switch } from '$lib/components/ui/switch';
	import * as Select from '$lib/components/ui/select';
	import type { VMCreateSettings } from '$lib/types/vm-create';
	import { DISK_BUSES } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
	}

	let { store, settings }: Props = $props();
</script>

<h2 class="text-lg font-semibold">{$t('vmCreate.hardware.title')}</h2>
<Separator />
<div class="grid gap-4 sm:grid-cols-2">
	<div class="space-y-2">
		<Label for="vm-sockets">{$t('vmCreate.hardware.sockets')}</Label>
		<Input
			id="vm-sockets"
			type="number"
			bind:value={store.form.sockets}
			min={settings.limits.sockets.min}
			max={settings.limits.sockets.max}
		/>
		<p class="text-muted-foreground text-xs">
			{settings.limits.sockets.min} – {settings.limits.sockets.max}
		</p>
	</div>
	<div class="space-y-2">
		<Label for="vm-cores">{$t('vmCreate.hardware.cores')}</Label>
		<Input
			id="vm-cores"
			type="number"
			bind:value={store.form.cores}
			min={settings.limits.cores.min}
			max={settings.limits.cores.max}
		/>
		<p class="text-muted-foreground text-xs">
			{settings.limits.cores.min} – {settings.limits.cores.max}
		</p>
	</div>
	<div class="space-y-2">
		<Label for="vm-memory">{$t('vmCreate.hardware.memory')}</Label>
		<Input
			id="vm-memory"
			type="number"
			bind:value={store.form.memoryGB}
			min={settings.limits.ram.min}
			max={settings.limits.ram.max}
		/>
		<p class="text-muted-foreground text-xs">
			{settings.limits.ram.min} – {settings.limits.ram.max} GB
		</p>
	</div>
	<div class="flex items-end pb-6">
		<Badge variant="secondary" class="text-sm">
			{$t('vmCreate.hardware.totalVCPUs', { values: { count: String(store.totalVCPUs) } })}
		</Badge>
	</div>
	<div class="space-y-2 sm:col-span-2">
		<Label>{$t('vmCreate.hardware.diskBus')}</Label>
		<Select.Root
			type="single"
			value={store.form.diskBus}
			onValueChange={(v) => (store.form.diskBus = v)}
		>
			<Select.Trigger class="w-full">
				{$t(DISK_BUSES.find((b) => b.value === store.form.diskBus)?.labelKey) ||
					store.form.diskBus}
			</Select.Trigger>
			<Select.Content>
				{#each DISK_BUSES as bus}
					<Select.Item value={bus.value}>{$t(bus.labelKey)}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
	<div class="flex items-center gap-3 sm:col-span-2">
		<Switch bind:checked={store.form.enableEFI} />
		<div>
			<p class="text-sm font-medium">{$t('vmCreate.hardware.enableEFI')}</p>
			<p class="text-muted-foreground text-xs">
				{$t('vmCreate.hardware.efiHint')}
			</p>
		</div>
	</div>
	<div class="flex items-center gap-3 sm:col-span-2">
		<Switch bind:checked={store.form.enableTPM} />
		<div>
			<p class="text-sm font-medium">{$t('vmCreate.hardware.enableTPM')}</p>
			<p class="text-muted-foreground text-xs">
				{$t('vmCreate.hardware.tpmHint')}
			</p>
		</div>
	</div>
</div>
