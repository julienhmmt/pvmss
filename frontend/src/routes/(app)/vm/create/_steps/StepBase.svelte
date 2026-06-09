<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Badge } from '$lib/components/ui/badge';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as Select from '$lib/components/ui/select';
	import type { VMCreateSettings } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
	}

	let { store, settings }: Props = $props();
</script>

<h2 class="text-lg font-semibold">{$t('vmCreate.base.title')}</h2>
<Separator />
<div class="grid gap-4 sm:grid-cols-2">
	<div class="space-y-2 sm:col-span-2">
		<Label for="vm-name">{$t('vmCreate.base.name')}</Label>
		<Input
			id="vm-name"
			bind:value={store.form.name}
			placeholder={$t('vmCreate.base.namePlaceholder')}
		/>
		<p
			class="text-xs {store.form.name.trim().length > 0 && !store.isNameValid
				? 'text-destructive'
				: 'text-muted-foreground'}"
		>
			{$t('vmCreate.base.nameHint')}
		</p>
	</div>
	<div class="space-y-2">
		<Label>{$t('vmCreate.base.node')}</Label>
		<Select.Root
			type="single"
			value={store.form.node}
			onValueChange={(v) => (store.form.node = v)}
		>
			<Select.Trigger class="w-full">
				{store.form.node || $t('vmCreate.base.selectNode')}
			</Select.Trigger>
			<Select.Content>
				{#each settings.nodes as node}
					<Select.Item value={node.name} disabled={node.disabled}>
						{node.name}
						{#if node.disabled}
							<span class="text-muted-foreground text-xs"> ({node.reason})</span>
						{/if}
					</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
		{#if settings.nodes.every((n) => n.disabled)}
			<div class="border-warning-soft-border bg-warning-soft rounded-lg border p-3 text-sm">
				<p class="text-warning-soft-foreground font-medium">{$t('vmCreate.base.allNodesDisabled')}</p>
				<p class="text-warning-soft-foreground/80 mt-1 text-xs">{$t('vmCreate.base.allNodesDisabledHint')}</p>
			</div>
		{/if}
	</div>
	<div class="space-y-2">
		<Label>{$t('vmCreate.base.storage')}</Label>
		<Select.Root
			type="single"
			value={store.form.storage}
			onValueChange={(v) => (store.form.storage = v)}
		>
			<Select.Trigger class="w-full">
				{store.form.storage || $t('vmCreate.base.selectStorage')}
			</Select.Trigger>
			<Select.Content>
				{#each settings.storages as storage}
					<Select.Item value={storage.name}>
						{storage.name}
						{#if storage.node}
							<span class="text-muted-foreground text-xs"> ({storage.node})</span>
						{/if}
					</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
		{#if settings.storages.length === 0}
			<div class="border-warning-soft-border bg-warning-soft rounded-lg border p-3 text-sm">
				<p class="text-warning-soft-foreground font-medium">{$t('vmCreate.base.noStorages')}</p>
				<p class="text-warning-soft-foreground/80 mt-1 text-xs">{$t('vmCreate.base.noStoragesHint')}</p>
			</div>
		{/if}
	</div>
	<div class="space-y-2 sm:col-span-2">
		<Label>{$t('vmCreate.base.iso')}</Label>
		<Select.Root
			type="single"
			value={store.form.iso}
			onValueChange={(v) => (store.form.iso = v ?? '')}
		>
			<Select.Trigger class="w-full">
				{#if store.form.iso}
					{settings.isos.find((i) => i.volid === store.form.iso)?.name || store.form.iso}
				{:else}
					{$t('vmCreate.base.noIso')}
				{/if}
			</Select.Trigger>
			<Select.Content>
				<Select.Item value="">{$t('vmCreate.base.noIso')}</Select.Item>
				{#each settings.isos as iso}
					<Select.Item value={iso.volid}>{iso.name}</Select.Item>
				{/each}
			</Select.Content>
		</Select.Root>
	</div>
	<div class="space-y-2 sm:col-span-2">
		<Label for="vm-desc">{$t('vmCreate.base.description')}</Label>
		<Textarea
			id="vm-desc"
			bind:value={store.form.description}
			placeholder={$t('vmCreate.base.descriptionPlaceholder')}
			rows={2}
		/>
	</div>
	{#if settings.tags.length > 0}
		<div class="space-y-2 sm:col-span-2">
			<Label>{$t('vmCreate.base.tags')}</Label>
			<div class="flex flex-wrap gap-2">
				{#each settings.tags as tag}
					<button
						type="button"
						onclick={() => store.toggleTag(tag)}
						class="cursor-pointer"
					>
						<Badge variant={store.form.tags.includes(tag) ? 'default' : 'outline'}>
							{tag}
						</Badge>
					</button>
				{/each}
			</div>
		</div>
	{/if}
</div>
