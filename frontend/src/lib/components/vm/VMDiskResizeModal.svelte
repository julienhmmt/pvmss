<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import * as Dialog from '$lib/components/ui/dialog';
	import { resizeDisk, type DiskInfo } from '$lib/api/vm-details';

	interface Props {
		open: boolean;
		vmid: number;
		disk: DiskInfo | null;
		maxDiskGB: number;
		onclose: () => void;
		onsuccess: () => void;
	}

	let { open = $bindable(), vmid, disk, maxDiskGB = 2000, onclose, onsuccess }: Props = $props();

	let addGB = $state(10);
	let saving = $state(false);

	const newTotal = $derived(disk ? disk.sizeGb + addGB : addGB);
	const exceedsMax = $derived(newTotal > maxDiskGB);

	$effect(() => {
		if (open) {
			addGB = 10;
		}
	});

	async function submit() {
		if (!disk) return;
		if (addGB <= 0) {
			toast.error($t('vm.disk.resizePositiveOnly'));
			return;
		}
		if (newTotal > maxDiskGB) {
			toast.error($t('vm.disk.sizeOutOfRange', { values: { min: disk.sizeGb, max: maxDiskGB } }));
			return;
		}
		saving = true;
		try {
			await resizeDisk(vmid, disk.index, addGB);
			toast.success($t('vm.disk.resizeSuccess', { values: { disk: disk.index } }));
			open = false;
			onsuccess();
		} catch {
			toast.error($t('vm.disk.resizeFailed'));
		} finally {
			saving = false;
		}
	}
</script>

<Dialog.Root bind:open onOpenChange={(v) => { if (!v) onclose(); }}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('vm.disk.resizeTitle')}</Dialog.Title>
			<Dialog.Description>
				{$t('vm.disk.resizeDescription', { values: { disk: disk?.index ?? '' } })}
			</Dialog.Description>
		</Dialog.Header>

		<div class="flex flex-col gap-4 py-2">
			{#if disk}
				<div class="rounded-md bg-muted px-3 py-2 text-sm">
					<span class="font-medium">{$t('vm.disk.currentSize')}:</span>
					{disk.sizeGb} {$t('common.gb')}
				</div>
			{/if}

			<div class="flex flex-col gap-1">
				<label class="text-sm font-medium" for="disk-add-gb">{$t('vm.disk.addGB')}</label>
				<input
					id="disk-add-gb"
					type="number"
					min="1"
					bind:value={addGB}
					class="rounded-md border border-input bg-background px-3 py-2 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				/>
				<p class="text-xs text-muted-foreground">
					{$t('vm.disk.newTotal')}: {newTotal} {$t('common.gb')} {exceedsMax ? `(max: ${maxDiskGB} {$t('common.gb')})` : ''}
				</p>
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
				disabled={saving || exceedsMax}
				class="inline-flex items-center rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90 disabled:opacity-50"
			>
				{saving ? $t('common.saving') : $t('vm.disk.resize')}
			</button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>
