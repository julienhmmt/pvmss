<script lang="ts">
	import type { AdminProfile } from './profiles.svelte';

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

<section class="mx-auto w-full max-w-4xl px-4 py-8">
	<div class="mb-6 flex items-center justify-between">
		<h1 class="text-2xl font-semibold tracking-tight">VM Profiles</h1>
		<button
			type="button"
			class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
			onclick={openCreate}
		>
			New profile
		</button>
	</div>

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

		<div class="overflow-x-auto rounded-lg border">
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
						<tr class="border-t">
							<td class="px-4 py-2 font-mono text-xs">{profile.id}</td>
							<td class="px-4 py-2">{profile.label}</td>
							<td class="px-4 py-2">{profile.cpuCores}</td>
							<td class="px-4 py-2">{profile.memoryMB} MB</td>
							<td class="px-4 py-2">{profile.diskGB} GB</td>
							<td class="px-4 py-2">{profile.bus}</td>
							<td class="px-4 py-2">
								<button
									type="button"
									class="rounded-md px-3 py-1 text-xs font-medium transition-colors {profile.enabled
										? 'bg-primary text-primary-foreground'
										: 'bg-muted text-muted-foreground hover:bg-muted/80'}"
									onclick={() => onToggle(profile.id, !profile.enabled)}
								>
									{profile.enabled ? 'Enabled' : 'Disabled'}
								</button>
							</td>
							<td class="px-4 py-2">
								<div class="flex gap-2">
									<button
										type="button"
										class="text-xs text-muted-foreground hover:text-foreground"
										onclick={() => openEdit(profile)}
									>
										Edit
									</button>
									<button
										type="button"
										class="text-xs text-destructive hover:text-destructive/80"
										onclick={() => onDelete(profile.id)}
									>
										Delete
									</button>
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
						<button
							type="button"
							class="rounded-md px-4 py-2 text-sm text-muted-foreground hover:text-foreground"
							onclick={() => (showForm = false)}
						>
							Cancel
						</button>
						<button
							type="submit"
							class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground hover:bg-primary/90"
							disabled={saving}
						>
							{saving ? 'Saving…' : editingId ? 'Save' : 'Create'}
						</button>
					</div>
				</form>
			</div>
		</div>
	{/if}
</section>
