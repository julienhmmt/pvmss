<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Monitor, Cpu, HardDrive, WifiHigh, Cloud, Warning } from 'phosphor-svelte';
	import { Separator } from '$lib/components/ui/separator';
	import { Switch } from '$lib/components/ui/switch';
	import type { VMCreateSettings } from '$lib/types/vm-create';
	import { DISK_BUSES, NETWORK_MODELS } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
	}

	let { store, settings }: Props = $props();

	const form = $derived(store.form);
	const hasErrors = $derived(
		!store.isStepValid('base') ||
			!store.isStepValid('hardware') ||
			!store.isStepValid('disk') ||
			!store.isStepValid('network') ||
			!store.isStepValid('cloudinit')
	);
	const diskBusEntry = $derived(DISK_BUSES.find((b) => b.value === form.diskBus));
</script>

<h2 class="text-lg font-semibold">{$t('vmCreate.review.title')}</h2>
<p class="text-muted-foreground text-sm">{$t('vmCreate.review.subtitle')}</p>

<!-- Validation warnings -->
{#if hasErrors}
	<div class="border-destructive/50 bg-destructive/5 rounded-lg border p-4">
		<div class="flex items-center gap-2">
			<Warning class="text-destructive h-5 w-5 flex-shrink-0" />
			<p class="text-destructive text-sm font-medium">
				{$t('vmCreate.review.validationTitle')}
			</p>
		</div>
		<ul class="text-destructive/80 mt-2 space-y-1 pl-7 text-sm">
			{#if form.name.trim().length === 0}
				<li>{$t('vmCreate.review.missingName')}</li>
			{:else if !store.isNameValid}
				<li>{$t('vmCreate.review.invalidName')}</li>
			{/if}
			{#if form.node === ''}
				<li>{$t('vmCreate.review.missingNode')}</li>
			{/if}
			{#if form.storage === ''}
				<li>{$t('vmCreate.review.missingStorage')}</li>
			{/if}
			{#if !store.isStepValid('hardware')}
				<li>{$t('vmCreate.review.invalidHardware')}</li>
			{/if}
			{#if !store.isStepValid('disk')}
				<li>{$t('vmCreate.review.invalidDisk')}</li>
			{/if}
			{#if !store.isStepValid('network')}
				<li>{$t('vmCreate.review.invalidNetwork')}</li>
			{/if}
			{#if !store.isStepValid('cloudinit')}
				<li>{$t('vmCreate.review.invalidCloudInit')}</li>
			{/if}
		</ul>
	</div>
{/if}

<Separator />

<!-- Base -->
<div class="space-y-2">
	<h3 class="flex items-center gap-2 text-sm font-semibold">
		<Monitor class="h-4 w-4" />
		{$t('vmCreate.review.base')}
	</h3>
	<div class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm">
		<span class="text-muted-foreground">{$t('vmCreate.review.name')}</span>
		<span class="font-medium {form.name.trim().length === 0 ? 'text-destructive' : ''}">
			{form.name.trim().length > 0 ? form.name : $t('vmCreate.review.required')}
		</span>
		<span class="text-muted-foreground">{$t('vmCreate.review.node')}</span>
		<span class={form.node === '' ? 'text-destructive' : ''}>
			{form.node || $t('vmCreate.review.required')}
		</span>
		<span class="text-muted-foreground">{$t('vmCreate.review.storage')}</span>
		<span class={form.storage === '' ? 'text-destructive' : ''}>
			{form.storage || $t('vmCreate.review.required')}
		</span>
		<span class="text-muted-foreground">{$t('vmCreate.review.iso')}</span>
		<span>
			{#if form.iso}
				{settings.isos.find((i) => i.volid === form.iso)?.name || form.iso}
			{:else}
				{$t('vmCreate.review.noIso')}
			{/if}
		</span>
	</div>
</div>

<!-- Hardware -->
<div class="space-y-2">
	<h3 class="flex items-center gap-2 text-sm font-semibold">
		<Cpu class="h-4 w-4" />
		{$t('vmCreate.review.hardware')}
	</h3>
	<div class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm">
		<span class="text-muted-foreground">{$t('vmCreate.review.sockets')}</span>
		<span>{form.sockets}</span>
		<span class="text-muted-foreground">{$t('vmCreate.review.cores')}</span>
		<span>{form.cores} ({store.totalVCPUs} {$t('common.vcpuCount', { values: { count: store.totalVCPUs } })})</span>
		<span class="text-muted-foreground">{$t('vmCreate.review.memory')}</span>
		<span>{form.memoryGB} {$t('common.gb')}</span>
		<span class="text-muted-foreground">{$t('vmCreate.review.diskBus')}</span>
		<span>{diskBusEntry ? $t(diskBusEntry.labelKey) : form.diskBus}</span>
		<span class="text-muted-foreground">{$t('vmCreate.review.efi')}</span>
		<span>{form.enableEFI ? $t('common.yes') : $t('common.no')}</span>
		<span class="text-muted-foreground">{$t('vmCreate.review.tpm')}</span>
		<span>{form.enableTPM ? $t('common.yes') : $t('common.no')}</span>
	</div>
</div>

<!-- Disks -->
<div class="space-y-2">
	<h3 class="flex items-center gap-2 text-sm font-semibold">
		<HardDrive class="h-4 w-4" />
		{$t('vmCreate.review.disk')}
	</h3>
	<div class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm">
		{#each form.disks as disk, i (i)}
			<span class="text-muted-foreground">
				{i === 0
					? $t('vmCreate.disk.primaryDisk')
					: $t('vmCreate.disk.diskIndex', { values: { index: String(i + 1) } })}
			</span>
			<span>{disk.sizeGb} {$t('common.gb')}</span>
		{/each}
		<span class="text-muted-foreground font-medium"
			>{$t('vmCreate.review.totalDisk')}</span
		>
		<span class="font-medium">{store.totalDiskGB} {$t('common.gb')}</span>
	</div>
</div>

<!-- Networks -->
<div class="space-y-2">
	<h3 class="flex items-center gap-2 text-sm font-semibold">
		<WifiHigh class="h-4 w-4" />
		{$t('vmCreate.review.network')}
	</h3>
	{#each form.networks as net, i (i)}
		{@const networkModelEntry = NETWORK_MODELS.find((m) => m.value === net.model)}
		<div class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm">
			<span class="text-muted-foreground">
				{$t('vmCreate.network.card', { values: { index: String(i + 1) } })}
			</span>
			<span></span>
			<span class="text-muted-foreground">{$t('vmCreate.review.bridge')}</span>
			<span class={net.bridge === '' ? 'text-destructive' : ''}>
				{#if net.bridge}
					{net.bridge}
					{@const desc = settings.bridges.find((b) => b.name === net.bridge)?.description}
					{#if desc}
						<span class="text-muted-foreground text-xs"> — {desc}</span>
					{/if}
				{:else}
					{$t('vmCreate.review.required')}
				{/if}
			</span>
			<span class="text-muted-foreground">{$t('vmCreate.review.model')}</span>
			<span>
				{networkModelEntry ? $t(networkModelEntry.labelKey) : net.model}
			</span>
			{#if net.vlan}
				<span class="text-muted-foreground">{$t('common.vlan')}</span>
				<span>{net.vlan}</span>
			{/if}
		</div>
	{/each}
</div>

<!-- Cloud-init -->
{#if form.cloudInitEnabled}
	<div class="space-y-2">
		<h3 class="flex items-center gap-2 text-sm font-semibold">
			<Cloud class="h-4 w-4" />
			{$t('vmCreate.review.cloudinit')}
		</h3>
		<div class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm">
			{#if form.ciUser}
				<span class="text-muted-foreground">{$t('vmCreate.cloudinit.user')}</span>
				<span>{form.ciUser}</span>
			{/if}
			<span class="text-muted-foreground">{$t('vmCreate.cloudinit.ipConfig')}</span>
			<span class="uppercase">{form.ciIPConfig}</span>
			{#if form.ciIPConfig === 'static' && form.ciIP}
				<span class="text-muted-foreground">{$t('common.ip')}</span>
				<span>{form.ciIP}</span>
			{/if}
			{#if form.ciTemplateID}
				<span class="text-muted-foreground">
					{$t('vmCreate.cloudinit.template')}
				</span>
				<span>
					{settings.cloudinitTemplates.find((t) => t.id === form.ciTemplateID)?.name ||
						form.ciTemplateID}
				</span>
			{/if}
		</div>
	</div>
{/if}

<Separator />

<!-- Start after creation -->
<div class="flex items-center gap-3">
	<Switch bind:checked={store.form.startAfterCreation} />
	<span class="text-sm font-medium">{$t('vmCreate.review.startAfterCreation')}</span>
</div>
