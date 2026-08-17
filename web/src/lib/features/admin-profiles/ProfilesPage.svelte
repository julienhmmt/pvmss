<script lang="ts">
	import type { AdminProfile } from './profiles.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		profiles: AdminProfile[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		onCreate: (label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string) => void;
		onUpdate: (id: string, label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string) => void;
		onDelete: (id: string) => void;
		onToggle: (id: string, enabled: boolean) => void;
	}

	let {
		profiles,
		loading,
		error,
		saving,
		saveError,
		onCreate,
		onUpdate,
		onDelete,
		onToggle
	}: Props = $props();

	let showForm = $state(false);
	let editingId = $state<string | null>(null);
	let pendingDelete = $state<AdminProfile | null>(null);
	let label = $state('');
	let cpuCores = $state(1);
	let memoryMB = $state(2048);
	let diskGB = $state(20);
	let bus = $state('scsi');

	function openCreate(): void {
		editingId = null;
		label = '';
		cpuCores = 1;
		memoryMB = 2048;
		diskGB = 20;
		bus = 'scsi';
		showForm = true;
	}

	function openEdit(profile: AdminProfile): void {
		editingId = profile.id;
		label = profile.label;
		cpuCores = profile.cpuCores;
		memoryMB = profile.memoryMB;
		diskGB = profile.diskGB;
		bus = profile.bus;
		showForm = true;
	}

	function submitForm(): void {
		if (editingId) {
			onUpdate(editingId, label, cpuCores, memoryMB, diskGB, bus);
		} else {
			onCreate(label, cpuCores, memoryMB, diskGB, bus);
		}
		showForm = false;
	}
</script>

<svelte:head>
	<title>{m['admin.profiles.pageTitle']()}</title>
</svelte:head>

<PageHeader title={m['admin.profiles.title']()}>
	{#snippet actions()}
		<Button onclick={openCreate}>{m['admin.profiles.newProfile']()}</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={8} />
{:else if error}
	<p role="alert" class="text-destructive">{error}</p>
{:else}
	{#if saveError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{saveError}
		</p>
	{/if}

	<div class="overflow-x-auto rounded-lg border border-border">
		<table class="w-full text-sm">
			<thead class="bg-muted/50 text-left">
				<tr>
					<th class="px-4 py-2 font-medium">{m['admin.profiles.id']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.profiles.labelField']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.profiles.vcpu']()}</th>
					<th class="px-4 py-2 font-medium">{m['common.memory']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.profiles.disk']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.profiles.bus']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.profiles.enabledStatus']()}</th>
					<th class="px-4 py-2 font-medium">{m['common.actions']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each profiles as profile (profile.id)}
					<tr class="border-t border-border">
						<td class="px-4 py-2 font-mono text-xs">{profile.id}</td>
						<td class="px-4 py-2">{profile.label}</td>
						<td class="px-4 py-2">{profile.cpuCores}</td>
						<td class="px-4 py-2">{profile.memoryMB} MB</td>
						<td class="px-4 py-2">{profile.diskGB} GB</td>
						<td class="px-4 py-2">{profile.bus}</td>
						<td class="px-4 py-2">
							<span class="inline-flex items-center gap-2">
								<Switch
									checked={profile.enabled}
									label={profile.enabled ? m['admin.profiles.disable']({ label: profile.label }) : m['admin.profiles.enable']({ label: profile.label })}
									onToggle={() => onToggle(profile.id, !profile.enabled)}
								/>
								<span class="text-xs text-muted-foreground">
									{profile.enabled ? m['admin.profiles.enabledStatus']() : m['admin.profiles.disabledStatus']()}
								</span>
							</span>
						</td>
						<td class="px-4 py-2">
							<div class="flex gap-2">
								<Button variant="ghost" size="sm" label={m['admin.profiles.editLabel']({ label: profile.label })} onclick={() => openEdit(profile)}>{m['admin.profiles.edit']()}</Button>
								<Button variant="destructive" size="sm" label={m['admin.profiles.deleteLabel']({ label: profile.label })} onclick={() => (pendingDelete = profile)}>{m['admin.profiles.delete']()}</Button>
							</div>
						</td>
					</tr>
				{:else}
					<tr><td colspan={8} class="p-0">
						<EmptyState title={m['admin.profiles.noProfiles']()}>
							{#snippet actions()}
								<Button onclick={openCreate}>{m['admin.profiles.newProfile']()}</Button>
							{/snippet}
						</EmptyState>
					</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<Dialog bind:open={showForm} size="lg" labelledBy="profile-form-title" onClose={() => (showForm = false)}>
	<h2 id="profile-form-title" class="mb-4 text-lg font-medium">{editingId ? m['admin.profiles.editProfile']() : m['admin.profiles.newProfileForm']()}</h2>
	<form onsubmit={(e) => { e.preventDefault(); submitForm(); }} class="space-y-4">
		<div>
			<label for="profile-label" class="mb-1 block text-sm font-medium">{m['admin.profiles.labelField']()}</label>
			<input
				id="profile-label"
				type="text"
				class="pv-input"
				bind:value={label}
				required
			/>
		</div>
		<div class="grid grid-cols-2 gap-4">
			<div>
				<label for="profile-cpu" class="mb-1 block text-sm font-medium">{m['admin.profiles.vcpuCores']()}</label>
				<input
					id="profile-cpu"
					type="number"
					min="1"
					class="pv-input"
					bind:value={cpuCores}
					required
				/>
			</div>
			<div>
				<label for="profile-mem" class="mb-1 block text-sm font-medium">{m['admin.profiles.memoryMb']()}</label>
				<input
					id="profile-mem"
					type="number"
					min="128"
					class="pv-input"
					bind:value={memoryMB}
					required
				/>
			</div>
			<div>
				<label for="profile-disk" class="mb-1 block text-sm font-medium">{m['admin.profiles.diskGb']()}</label>
				<input
					id="profile-disk"
					type="number"
					min="1"
					class="pv-input"
					bind:value={diskGB}
					required
				/>
			</div>
			<div>
				<label for="profile-bus" class="mb-1 block text-sm font-medium">{m['admin.profiles.busField']()}</label>
				<select id="profile-bus" class="pv-input pv-select" bind:value={bus}>
					<option value="scsi">scsi</option>
					<option value="virtio">virtio</option>
					<option value="sata">sata</option>
					<option value="ide">ide</option>
				</select>
			</div>
		</div>
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="ghost" onclick={() => (showForm = false)}>{m['common.cancel']()}</Button>
			<Button type="submit" disabled={saving}>
				{saving ? m['common.saving']() : editingId ? m['common.save']() : m['common.create']()}
			</Button>
		</div>
	</form>
</Dialog>

<ConfirmDialog
	open={pendingDelete !== null}
	title={m['admin.profiles.deleteTitle']({ label: pendingDelete?.label ?? '' })}
	message={m['admin.profiles.deleteConfirm']()}
	confirmLabel={m['common.deletePermanently']()}
	cancelLabel={m['common.cancel']()}
	confirming={saving}
	testId="profile-delete-confirm"
	onConfirm={() => { if (pendingDelete) { onDelete(pendingDelete.id); pendingDelete = null; } }}
	onClose={() => (pendingDelete = null)}
/>
