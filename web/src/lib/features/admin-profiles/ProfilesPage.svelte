<script lang="ts">
	import type { AdminProfile } from './profiles.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';

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
	<title>Admin Profiles — PVMSS</title>
</svelte:head>

<PageHeader title="VM Profiles">
	{#snippet actions()}
		<Button onclick={openCreate}>New profile</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
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
					<th class="px-4 py-2 font-medium">ID</th>
					<th class="px-4 py-2 font-medium">Label</th>
					<th class="px-4 py-2 font-medium">vCPU</th>
					<th class="px-4 py-2 font-medium">Memory</th>
					<th class="px-4 py-2 font-medium">Disk</th>
					<th class="px-4 py-2 font-medium">Bus</th>
					<th class="px-4 py-2 font-medium">Enabled</th>
					<th class="px-4 py-2 font-medium">Actions</th>
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
									label={profile.enabled ? `Disable ${profile.label}` : `Enable ${profile.label}`}
									onToggle={() => onToggle(profile.id, !profile.enabled)}
								/>
								<span class="text-xs text-muted-foreground">
									{profile.enabled ? 'Enabled' : 'Disabled'}
								</span>
							</span>
						</td>
						<td class="px-4 py-2">
							<div class="flex gap-2">
								<Button variant="ghost" size="sm" label={`Edit ${profile.label}`} onclick={() => openEdit(profile)}>Edit</Button>
								<Button variant="destructive" size="sm" label={`Delete ${profile.label}`} onclick={() => onDelete(profile.id)}>Delete</Button>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

{#if showForm}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" role="dialog" aria-modal="true">
		<div class="w-full max-w-md rounded-lg bg-background p-6 shadow-lg">
			<h2 class="mb-4 text-lg font-medium">{editingId ? 'Edit profile' : 'New profile'}</h2>
			<form onsubmit={(e) => { e.preventDefault(); submitForm(); }} class="space-y-4">
				<div>
					<label for="profile-label" class="mb-1 block text-sm font-medium">Label</label>
					<input
						id="profile-label"
						type="text"
						class="w-full rounded-md border bg-background px-3 py-2 text-sm"
						bind:value={label}
						required
					/>
				</div>
				<div class="grid grid-cols-2 gap-4">
					<div>
						<label for="profile-cpu" class="mb-1 block text-sm font-medium">vCPU cores</label>
						<input
							id="profile-cpu"
							type="number"
							min="1"
							class="w-full rounded-md border bg-background px-3 py-2 text-sm"
							bind:value={cpuCores}
							required
						/>
					</div>
					<div>
						<label for="profile-mem" class="mb-1 block text-sm font-medium">Memory (MB)</label>
						<input
							id="profile-mem"
							type="number"
							min="128"
							class="w-full rounded-md border bg-background px-3 py-2 text-sm"
							bind:value={memoryMB}
							required
						/>
					</div>
					<div>
						<label for="profile-disk" class="mb-1 block text-sm font-medium">Disk (GB)</label>
						<input
							id="profile-disk"
							type="number"
							min="1"
							class="w-full rounded-md border bg-background px-3 py-2 text-sm"
							bind:value={diskGB}
							required
						/>
					</div>
					<div>
						<label for="profile-bus" class="mb-1 block text-sm font-medium">Bus</label>
						<select id="profile-bus" class="w-full rounded-md border bg-background px-3 py-2 text-sm" bind:value={bus}>
							<option value="scsi">scsi</option>
							<option value="virtio">virtio</option>
							<option value="sata">sata</option>
							<option value="ide">ide</option>
						</select>
					</div>
				</div>
				<div class="flex justify-end gap-2 pt-2">
					<Button variant="ghost" onclick={() => (showForm = false)}>Cancel</Button>
					<Button type="submit" disabled={saving}>
						{saving ? 'Saving…' : editingId ? 'Save' : 'Create'}
					</Button>
				</div>
			</form>
		</div>
	</div>
{/if}
