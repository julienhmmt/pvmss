<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import * as AlertDialog from '$lib/components/ui/alert-dialog';
	import { deleteDisk, type DiskInfo } from '$lib/api/vm-details';

	interface Props {
		open: boolean;
		vmid: number;
		disk: DiskInfo | null;
		vmStatus: string;
		onclose: () => void;
		onsuccess: () => void;
	}

	let { open = $bindable(), vmid, disk, vmStatus = 'stopped', onclose, onsuccess }: Props = $props();

	let deleting = $state(false);

	async function confirm() {
		if (!disk) return;
		deleting = true;
		try {
			await deleteDisk(vmid, disk.index);
			toast.success($t('vm.disk.deleteSuccess', { values: { disk: disk.index } }));
			open = false;
			onsuccess();
		} catch {
			toast.error($t('vm.disk.deleteFailed'));
		} finally {
			deleting = false;
		}
	}
</script>

<AlertDialog.Root bind:open>
	<AlertDialog.Content>
		<AlertDialog.Header>
			<AlertDialog.Title>{$t('vm.disk.deleteTitle')}</AlertDialog.Title>
			<AlertDialog.Description>
				{$t('vm.disk.deleteDescription', { values: { disk: disk?.index ?? '' } })}
				<br /><br />
				{#if vmStatus === 'running'}
					<span class="font-medium text-destructive">{$t('vm.disk.vmRunningWarning')}</span>
					<br /><br />
				{/if}
				{#if disk?.isBoot}
					<span class="font-medium text-amber-600">{$t('vm.disk.bootDiskWarning')}</span>
					<br /><br />
				{/if}
				<span class="font-medium text-amber-600">{$t('vm.disk.deleteWarning')}</span>
			</AlertDialog.Description>
		</AlertDialog.Header>
		<AlertDialog.Footer>
			<AlertDialog.Cancel onclick={onclose}>{$t('common.cancel')}</AlertDialog.Cancel>
			<AlertDialog.Action
				onclick={confirm}
				disabled={deleting || vmStatus === 'running' || !!disk?.isBoot}
				class="bg-destructive text-destructive-foreground hover:bg-destructive/90 disabled:opacity-50"
			>
				{deleting ? $t('common.saving') : $t('vm.disk.detach')}
			</AlertDialog.Action>
		</AlertDialog.Footer>
	</AlertDialog.Content>
</AlertDialog.Root>
