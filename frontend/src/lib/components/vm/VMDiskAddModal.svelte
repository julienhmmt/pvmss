<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import * as Dialog from '$lib/components/ui/dialog';
	import { addDisk, type VMSettings } from '$lib/api/vm-details';

	const BUS_TYPES = ['virtio', 'scsi', 'sata', 'ide'];

	interface Props {
		open: boolean;
		vmid: number;
		settings: VMSettings | null;
		currentDiskCount: number;
		onclose: () => void;
		onsuccess: () => void;
	}

	let { open = $bindable(), vmid, settings, currentDiskCount = 0, onclose, onsuccess }: Props = $props();

	let storage = $state('');
	let sizeGB = $state(10);
	let bus = $state('virtio');
	let saving = $state(false);

	const storages = $derived(settings?.availableStorages ?? []);
	const storageNames = $derived<string[]>([...new Set(storages.map((s) => s.storage))]);
	const minDisk = $derived<number>(settings?.limits.minDiskGb ?? 1);
	const maxDisk = $derived<number>(settings?.limits.maxDiskGb ?? 2000);
	const maxDisksPerVM = $derived<number>(settings?.limits.maxDisksPerVm ?? 4);

	$effect(() => {
		if (open) {
			storage = storageNames[0] ?? '';
			sizeGB = minDisk > 0 ? minDisk : 10;
			bus = 'virtio';
		}
	});

	async function submit() {
		if (!storage) {
			toast.error($t('vm.disk.storageRequired'));
			return;
		}
		if (sizeGB < minDisk || sizeGB > maxDisk) {
			toast.error($t('vm.disk.sizeOutOfRange', { values: { min: minDisk, max: maxDisk } }));
			return;
		}
		if (currentDiskCount >= maxDisksPerVM) {
			toast.error($t('vm.disk.maxDisksReached', { values: { max: maxDisksPerVM } }));
			return;
		}
		saving = true;
		try {
			const result = await addDisk(vmid, { storage, sizeGb: sizeGB, bus });
			toast.success($t('vm.disk.addSuccess', { values: { disk: result.disk } }));
			open = false;
			onsuccess();
		} catch {
			toast.error($t('vm.disk.addFailed'));
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) onclose(); }}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('vm.disk.addTitle')}</Dialog.Title>
			<Dialog.Description>
				{$t('vm.disk.addDescription')}
				<span class="text-muted-foreground"> ({$t('vm.disk.currentMaxDisks', { values: { current: currentDiskCount, max: maxDisksPerVM } })})</span>
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex flex-col gap-4 py-2">
			<!-- Storage -->
			<div class="flex flex-col gap-1">
				<label class="text-sm font-medium" for="disk-storage">{$t('common.storage')}</label>
				{#if storageNames.length === 0}
					<p class="text-sm text-muted-foreground">{$t('vm.disk.noStorages')}</p>
				{:else}
					<select
						id="disk-storage"
						bind:value={storage}
						class="rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
					>
						{#each storageNames as name (name)}
							<option value={name}>{name}</option>
						{/each}
					</select>
				{/if}
			</div>

			<!-- Bus type -->
			<div class="flex flex-col gap-1">
				<label class="text-sm font-medium" for="disk-bus">{$t('vm.disk.busType')}</label>
				<select
					id="disk-bus"
					bind:value={bus}
					class="rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				>
					{#each BUS_TYPES as b (b)}
						<option value={b}>{b}</option>
					{/each}
				</select>
			</div>

			<!-- Size -->
			<div class="flex flex-col gap-1">
				<label class="text-sm font-medium" for="disk-size">
					{$t('vm.disk.size')} ({minDisk}–{maxDisk} {$t('common.gb')})
				</label>
				<input
					id="disk-size"
					type="number"
					min={minDisk}
					max={maxDisk}
					bind:value={sizeGB}
					class="rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				/>
			</div>
		</div>

		<Dialog.Footer>
			<button
				type="button"
				onclick={onclose}
				class="inline-flex items-center rounded-md border border-input bg-background px-4 py-2 text-sm font-medium hover:bg-accent"
			>
				{$t('common.cancel')}
			</button>
			<button
				type="button"
				onclick={submit}
				disabled={saving || storageNames.length === 0}
				class="inline-flex items-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			>
				{saving ? $t('common.saving') : $t('vm.disk.add')}
			</button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
