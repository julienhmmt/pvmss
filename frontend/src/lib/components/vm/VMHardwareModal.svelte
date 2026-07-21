<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import * as Dialog from '$lib/components/ui/dialog';
	import {
		updateVMHardware,
		type VMSettings,
		type NetworkUpdateRequest
	} from '$lib/api/vm-details';
	import type { NetworkInterface } from '$lib/api/vm-details';
	import { WarningCircle, Trash, Plus } from 'phosphor-svelte';

	const MANDATORY_TAG = 'pvmss';

	const NET_MODELS = ['virtio', 'e1000', 'e1000e', 'rtl8139', 'vmxnet3'];

	interface Props {
		open: boolean;
		vmid: number;
		node: string;
		currentSockets: number;
		currentCores: number;
		currentMemMB: number;
		currentTags: string;
		currentNetworks: NetworkInterface[];
		isRunning: boolean;
		settings: VMSettings | null;
		onclose: () => void;
		onsuccess: () => void;
	}

	let {
		open = $bindable(),
		vmid,
		node,
		currentSockets,
		currentCores,
		currentMemMB,
		currentTags,
		currentNetworks,
		isRunning,
		settings,
		onclose,
		onsuccess
	}: Props = $props();

	type TabKey = 'hardware' | 'network' | 'tags';
	let activeTab = $state<TabKey>('hardware');

	// ── Hardware state ──────────────────────────────────────────────────────────
	let sockets = $state(0);
	let cores = $state(0);
	let memGB = $state(0);

	// ── Tags state ──────────────────────────────────────────────────────────────
	let selectedTags = $state<string[]>([]);

	// ── Network state ──────────────────────────────────────────────────────────
	// Each entry is either an existing card (has index) or a new one (index='')
	interface NetCard {
		index: string;
		model: string;
		bridge: string;
		mac: string;
		vlan: number;
		rate: string;
		firewall: boolean;
		deleted: boolean;
	}
	let cards = $state<NetCard[]>([]);

	let saving = $state(false);

	const limits = $derived(
		settings?.limits ?? {
			minSockets: 1,
			maxSockets: 4,
			minCores: 1,
			maxCores: 16,
			minRamGb: 1,
			maxRamGb: 64
		}
	);

	const availableTags = $derived(
		(settings?.availableTags ?? []).filter((tag) => tag !== MANDATORY_TAG)
	);

	const availableBridges = $derived(
		(settings?.availableVmbrs ?? []).map((b) => b.iface)
	);

	const defaultBridge = $derived(availableBridges[0] ?? '');

	const activeCards = $derived(cards.filter((c) => !c.deleted));

	$effect(() => {
		if (open) {
			activeTab = 'hardware';
			sockets = currentSockets;
			cores = currentCores;
			memGB = Math.round(currentMemMB / 1024);

			selectedTags = (currentTags ?? '')
				.split(';')
				.map((t) => t.trim())
				.filter((t) => t && t !== MANDATORY_TAG);

			cards = currentNetworks.map((n) => ({
				index: n.index ?? '',
				model: n.model ?? 'virtio',
				bridge: n.bridge ?? defaultBridge,
				mac: n.mac ?? '',
				vlan: n.tag ?? 0,
				rate: n.rate ?? '',
				firewall: n.firewall ?? false,
				deleted: false
			}));
		}
	});

	function toggleTag(tag: string) {
		if (selectedTags.includes(tag)) {
			selectedTags = selectedTags.filter((t) => t !== tag);
		} else {
			selectedTags = [...selectedTags, tag];
		}
	}

	function addCard() {
		cards = [
			...cards,
			{
				index: '',
				model: 'virtio',
				bridge: defaultBridge,
				mac: '',
				vlan: 0,
				rate: '',
				firewall: false,
				deleted: false
			}
		];
	}

	function markDeleted(i: number) {
		cards = cards.map((c, idx) => (idx === i ? { ...c, deleted: true } : c));
	}

	function updateCard(i: number, field: keyof NetCard, value: NetCard[keyof NetCard]) {
		cards = cards.map((c, idx) => (idx === i ? { ...c, [field]: value } : c));
	}

	async function confirm() {
		saving = true;
		try {
			const tagsString = [MANDATORY_TAG, ...selectedTags].join(';');

			// Build network updates: existing changed cards + new cards
			// Empty index means "add new card" - backend will auto-assign net0, net1, etc.
			const networks: NetworkUpdateRequest[] = activeCards
				.map((c) => ({
					index: c.index,
					model: c.model,
					bridge: c.bridge,
					mac: c.mac,
					vlan: c.vlan,
					rate: c.rate,
					firewall: c.firewall
				}));

			// Deleted cards (existing ones that were marked deleted)
			const deleteNetworks = cards
				.filter((c) => c.deleted && c.index !== '')
				.map((c) => c.index);

			const result = await updateVMHardware(vmid, {
				node,
				sockets,
				cores,
				memoryMb: memGB * 1024,
				tags: tagsString,
				networks,
				deleteNetworks: deleteNetworks
			});

			if (result.restarted) {
				toast.success($t('vm.hardware.updatedRestarted'));
			} else {
				toast.success($t('vm.hardware.updated'));
			}
			open = false;
			onsuccess();
		} catch {
			toast.error($t('vm.hardware.failed'));
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root bind:open>
	<Dialog.Content class="max-w-lg">
		<Dialog.Header>
			<Dialog.Title>{$t('vm.hardware.title')}</Dialog.Title>
			<Dialog.Description>
				{$t('vm.hardware.description', { values: { vmid } })}
				{#if isRunning}
					<span class="mt-1 flex items-center gap-1 font-medium text-warning-soft-foreground">
						<WarningCircle class="h-4 w-4" />
						{$t('vm.hardware.warningRunning')}
					</span>
				{/if}
			</Dialog.Description>
		</Dialog.Header>

		<!-- Tabs -->
		<div class="flex gap-1 border-b border-border">
			{#each [
				{ key: 'hardware' as TabKey, label: $t('vm.hardware.title') },
				{ key: 'network' as TabKey, label: $t('vm.tabNetwork') },
				{ key: 'tags' as TabKey, label: $t('admin.vms.tags') }
			] as tab (tab.key)}
				<button
					class="border-b-2 px-3 py-2 text-sm transition-colors {activeTab === tab.key
						? 'border-primary text-foreground'
						: 'border-transparent text-muted-foreground hover:text-foreground'}"
					onclick={() => (activeTab = tab.key)}
				>
					{tab.label}
				</button>
			{/each}
		</div>

		<!-- Hardware tab -->
		{#if activeTab === 'hardware'}
			<div class="grid grid-cols-2 gap-4 py-2">
				<div>
					<label class="mb-1 block text-sm font-medium" for="hw-sockets">
						{$t('vm.hardware.sockets')}
					</label>
					<input
						id="hw-sockets"
						type="number"
						min={limits.minSockets}
						max={limits.maxSockets}
						bind:value={sockets}
						class="w-full rounded border border-border bg-background px-3 py-2 text-sm"
					/>
					<p class="mt-0.5 text-xs text-muted-foreground">{limits.minSockets}–{limits.maxSockets}</p>
				</div>
				<div>
					<label class="mb-1 block text-sm font-medium" for="hw-cores">
						{$t('vm.hardware.cores')}
					</label>
					<input
						id="hw-cores"
						type="number"
						min={limits.minCores}
						max={limits.maxCores}
						bind:value={cores}
						class="w-full rounded border border-border bg-background px-3 py-2 text-sm"
					/>
					<p class="mt-0.5 text-xs text-muted-foreground">{limits.minCores}–{limits.maxCores}</p>
				</div>
				<div class="col-span-2">
					<label class="mb-1 block text-sm font-medium" for="hw-memory">
						{$t('vm.hardware.memory')}
					</label>
					<input
						id="hw-memory"
						type="number"
						min={limits.minRamGb}
						max={limits.maxRamGb}
						bind:value={memGB}
						class="w-full rounded border border-border bg-background px-3 py-2 text-sm"
					/>
					<p class="mt-0.5 text-xs text-muted-foreground">{limits.minRamGb}–{limits.maxRamGb} {$t('common.gb')}</p>
				</div>
			</div>
		{/if}

		<!-- Network tab -->
		{#if activeTab === 'network'}
			<div class="space-y-3 py-2">
				{#each cards as card, i (i)}
					{#if !card.deleted}
						<div class="rounded border border-border p-3">
							<div class="mb-2 flex items-center justify-between">
								<span class="text-xs font-medium text-muted-foreground">
									{card.index || $t('vm.network.newCard')}
								</span>
								<button
									type="button"
									class="text-muted-foreground hover:text-destructive"
									title={$t('vm.network.deleteCard')}
									onclick={() => markDeleted(i)}
								>
									<Trash class="h-4 w-4" />
								</button>
							</div>
							<div class="grid grid-cols-2 gap-2">
								<!-- Model -->
								<div>
									<label class="mb-0.5 block text-xs font-medium" for={`hw-model-${i}`}>{$t('vm.network.model')}</label>
									<select
										id={`hw-model-${i}`}
										class="w-full rounded border border-border bg-background px-2 py-1.5 text-sm"
										value={card.model}
										onchange={(e: Event) => updateCard(i, 'model', (e.currentTarget as HTMLSelectElement).value)}
									>
										{#each NET_MODELS as m (m)}
											<option value={m}>{m}</option>
										{/each}
									</select>
								</div>
								<!-- Bridge -->
								<div>
									<label class="mb-0.5 block text-xs font-medium" for={`hw-bridge-${i}`}>{$t('vm.network.bridge')}</label>
									{#if availableBridges.length > 0}
										<select
											id={`hw-bridge-${i}`}
											class="w-full rounded border border-border bg-background px-2 py-1.5 text-sm"
											value={card.bridge}
											onchange={(e: Event) => updateCard(i, 'bridge', (e.currentTarget as HTMLSelectElement).value)}
										>
											{#each availableBridges as br (br)}
												<option value={br}>{br}</option>
											{/each}
										</select>
									{:else}
										<input
											id={`hw-bridge-${i}`}
											type="text"
											class="w-full rounded border border-border bg-background px-2 py-1.5 text-sm"
											value={card.bridge}
											oninput={(e: Event) => updateCard(i, 'bridge', (e.currentTarget as HTMLInputElement).value)}
										/>
									{/if}
								</div>
								<!-- VLAN -->
								<div>
									<label class="mb-0.5 block text-xs font-medium" for={`hw-vlan-${i}`}>{$t('vm.network.vlan')}</label>
									<input
										id={`hw-vlan-${i}`}
										type="number"
										min="0"
										max="4094"
										class="w-full rounded border border-border bg-background px-2 py-1.5 text-sm"
										value={card.vlan}
										oninput={(e: Event) => updateCard(i, 'vlan', parseInt((e.currentTarget as HTMLInputElement).value) || 0)}
									/>
									<p class="text-xs text-muted-foreground">{$t('vm.network.vlanHint')}</p>
								</div>
								<!-- Rate -->
								<div>
									<label class="mb-0.5 block text-xs font-medium" for={`hw-rate-${i}`}>{$t('vm.network.rate')}</label>
									<input
										id={`hw-rate-${i}`}
										type="text"
										class="w-full rounded border border-border bg-background px-2 py-1.5 text-sm"
										value={card.rate}
										oninput={(e: Event) => updateCard(i, 'rate', (e.currentTarget as HTMLInputElement).value)}
										placeholder="e.g. 10"
									/>
									<p class="text-xs text-muted-foreground">{$t('vm.network.rateHint')}</p>
								</div>
								<!-- Firewall -->
								<div class="col-span-2 flex items-center gap-2">
									<input
										id={`hw-fw-${i}`}
										type="checkbox"
										checked={card.firewall}
										onchange={(e: Event) => updateCard(i, 'firewall', (e.currentTarget as HTMLInputElement).checked)}
									/>
									<label class="text-sm" for={`hw-fw-${i}`}>{$t('vm.network.firewall')}</label>
								</div>
							</div>
						</div>
					{/if}
				{/each}

				{#if availableBridges.length === 0}
					<p class="text-center text-sm text-muted-foreground">{$t('vm.network.noVmbr')}</p>
				{:else}
					<button
						type="button"
						class="inline-flex items-center gap-1 text-sm text-primary hover:underline"
						onclick={addCard}
					>
						<Plus class="h-4 w-4" />
						{$t('vm.network.addCard')}
					</button>
				{/if}
			</div>
		{/if}

		<!-- Tags tab -->
		{#if activeTab === 'tags'}
			<div class="py-2">
				{#if availableTags.length === 0}
					<p class="text-sm text-muted-foreground">{$t('vm.hardware.tagsHint')}</p>
				{:else}
					<p class="mb-2 text-sm font-medium">{$t('vm.hardware.tags')}</p>
					<div class="flex flex-wrap gap-2">
						<span class="rounded-full bg-primary/15 px-3 py-1 text-xs font-medium text-primary">
							{MANDATORY_TAG}
						</span>
						{#each availableTags as tag (tag)}
							<button
								type="button"
								class="rounded-full border px-3 py-1 text-xs transition-colors {selectedTags.includes(
									tag
								)
									? 'border-primary bg-primary/15 text-primary'
									: 'border-border text-muted-foreground hover:border-primary/50 hover:text-foreground'}"
								onclick={() => toggleTag(tag)}
							>
								{tag}
							</button>
						{/each}
					</div>
					<p class="mt-2 text-xs text-muted-foreground">{$t('vm.hardware.tagsHint')}</p>
				{/if}
			</div>
		{/if}

		<Dialog.Footer>
			<button
				class="rounded border border-border px-4 py-2 text-sm hover:bg-muted"
				disabled={saving}
				onclick={onclose}
			>
				{$t('common.cancel')}
			</button>
			<button
				class="rounded bg-primary px-4 py-2 text-sm text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
				disabled={saving}
				onclick={confirm}
			>
				{saving ? $t('vm.hardware.applying') : $t('vm.hardware.applyChanges')}
			</button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
