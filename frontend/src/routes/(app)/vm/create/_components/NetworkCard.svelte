<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Trash } from 'phosphor-svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import * as Select from '$lib/components/ui/select';
	import type { VMCreateSettings } from '$lib/types/vm-create';
	import { NETWORK_MODELS } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
		index: number;
	}

	let { store, settings, index }: Props = $props();

	const net = $derived(store.form.networks[index]);
</script>

{#if net}
<div class="bg-muted/50 space-y-3 rounded-lg p-4">
	<div class="flex items-center justify-between">
		<h3 class="text-sm font-medium">
			{$t('vmCreate.network.card', { values: { index: String(index + 1) } })}
		</h3>
		{#if store.form.networks.length > 1}
			<Button
				variant="ghost"
				size="sm"
				aria-label={$t('common.remove')}
				onclick={() => store.removeNetworkCard(index)}
				class="text-destructive"
			>
				<Trash class="h-4 w-4" />
			</Button>
		{/if}
	</div>
	<div class="grid gap-3 sm:grid-cols-2">
		<div class="space-y-1">
			<Label>{$t('vmCreate.network.bridge')}</Label>
			<Select.Root
				type="single"
				value={net.bridge}
				onValueChange={(v) => store.updateNetwork(index, { bridge: v })}
			>
				<Select.Trigger class="w-full">
					{net.bridge || $t('vmCreate.network.selectBridge')}
				</Select.Trigger>
				<Select.Content>
					{#each settings.bridges as bridge, i (i)}
						<Select.Item value={bridge.name}>
							{bridge.name}
							{#if bridge.description}
								<span class="text-muted-foreground text-xs">
									— {bridge.description}</span
								>
							{/if}
						</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
		<div class="space-y-1">
			<Label>{$t('vmCreate.network.model')}</Label>
			<Select.Root
				type="single"
				value={net.model}
				onValueChange={(v) => store.updateNetwork(index, { model: v })}
			>
				<Select.Trigger class="w-full">
					{$t(NETWORK_MODELS.find((m) => m.value === net.model)?.labelKey) || net.model}
				</Select.Trigger>
				<Select.Content>
					{#each NETWORK_MODELS as model, i (i)}
						<Select.Item value={model.value}>{$t(model.labelKey)}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>
		</div>
		<div class="space-y-1">
			<Label>{$t('vmCreate.network.mac')}</Label>
			<Input
				value={net.mac}
				oninput={(e) => store.updateNetwork(index, { mac: (e.target as HTMLInputElement).value })}
				placeholder={$t('vmCreate.network.macPlaceholder')}
			/>
		</div>
		<div class="space-y-1">
			<Label>{$t('vmCreate.network.vlan')}</Label>
			<Input
				type="number"
				value={net.vlan || ''}
				oninput={(e) =>
					store.updateNetwork(index, {
						vlan: parseInt((e.target as HTMLInputElement).value) || 0
					})}
				placeholder={$t('vmCreate.network.vlanPlaceholder')}
				min={0}
				max={4096}
			/>
		</div>
	</div>
	<div class="flex items-center gap-3">
		<Switch
			checked={net.enabled}
			onCheckedChange={(v) => store.updateNetwork(index, { enabled: v })}
		/>
		<span class="text-sm">{$t('vmCreate.network.enabled')}</span>
	</div>
</div>
{/if}
