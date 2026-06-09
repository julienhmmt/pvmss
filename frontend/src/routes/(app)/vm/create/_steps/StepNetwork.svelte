<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Plus } from 'phosphor-svelte';
	import { Badge } from '$lib/components/ui/badge';
	import { Button } from '$lib/components/ui/button';
	import { Separator } from '$lib/components/ui/separator';
	import type { VMCreateSettings } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';
	import NetworkCard from '../_components/NetworkCard.svelte';

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
	}

	let { store, settings }: Props = $props();
</script>

<div class="flex items-center justify-between">
	<h2 class="text-lg font-semibold">{$t('vmCreate.network.title')}</h2>
	<Badge variant="secondary">
		{$t('vmCreate.network.cardCount', {
			values: { current: String(store.form.networks.length), max: String(settings.maxNetworkCards) }
		})}
	</Badge>
</div>
<Separator />
<div class="space-y-4">
	{#each store.form.networks as net, i (i)}
		<NetworkCard {store} {settings} index={i} />
	{/each}
	{#if store.form.networks.length < settings.maxNetworkCards}
		<Button variant="outline" size="sm" onclick={() => store.addNetworkCard()}>
			<Plus class="mr-1 h-4 w-4" />
			{$t('vmCreate.network.addCard')}
		</Button>
	{:else}
		<p class="text-muted-foreground text-xs">
			{$t('vmCreate.network.maxCardsReached', {
				values: { max: String(settings.maxNetworkCards) }
			})}
		</p>
	{/if}
</div>
