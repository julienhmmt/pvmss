<script lang="ts">
	import type { AdminProfile } from './profiles.svelte';
	import type { ClusterOption } from '$lib/shared/clusters';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import TooltipHeader from '$lib/shared/ui/TooltipHeader.svelte';
	import SortableTooltipHeader from '$lib/shared/ui/SortableTooltipHeader.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type ProfileSortColumn = 'id' | 'label' | 'cpuCores' | 'memoryMB' | 'diskGB';

	interface Props {
		profiles: AdminProfile[];
		filteredProfiles: AdminProfile[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		onClusterChange: (value: string) => void;
		search: string;
		busFilter: string;
		enabledFilter: 'all' | 'enabled' | 'disabled';
		busOptions: string[];
		sortBy: ProfileSortColumn;
		sortDir: 'asc' | 'desc';
		onSearchChange: (value: string) => void;
		onBusFilterChange: (value: string) => void;
		onEnabledFilterChange: (value: 'all' | 'enabled' | 'disabled') => void;
		onSort: (column: ProfileSortColumn) => void;
		onResetFilters: () => void;
		onCreate: (label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string) => void;
		onUpdate: (id: string, label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string) => void;
		onDelete: (id: string) => void;
		onToggle: (id: string, enabled: boolean) => void;
	}

	let {
		profiles,
		filteredProfiles,
		loading,
		error,
		saving,
		saveError,
		clusterOptions,
		cluster,
		onClusterChange,
		search,
		busFilter,
		enabledFilter,
		busOptions,
		sortBy,
		sortDir,
		onSearchChange,
		onBusFilterChange,
		onEnabledFilterChange,
		onSort,
		onResetFilters,
		onCreate,
		onUpdate,
		onDelete,
		onToggle
	}: Props = $props();

	function handleSort(column: string): void {
		onSort(column as ProfileSortColumn);
	}

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
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="profiles-cluster" />
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

	{#if profiles.length > 0}
		<div class="mb-4 flex flex-wrap items-center gap-2">
			<input
				type="search"
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
				placeholder={m['admin.profiles.searchPlaceholder']()}
				value={search}
				oninput={(e) => onSearchChange(e.currentTarget.value)}
			/>
			<select class="rounded-md border border-border bg-background px-3 py-1.5 text-sm" value={busFilter} onchange={(e) => onBusFilterChange(e.currentTarget.value)}>
				<option value="">{m['admin.profiles.filterBus']()}</option>
				{#each busOptions as bus (bus)}
					<option value={bus}>{bus}</option>
				{/each}
			</select>
			<select class="rounded-md border border-border bg-background px-3 py-1.5 text-sm" value={enabledFilter} onchange={(e) => onEnabledFilterChange(e.currentTarget.value as 'all' | 'enabled' | 'disabled')}>
				<option value="all">{m['admin.profiles.filterEnabled']()}</option>
				<option value="enabled">{m['admin.profiles.filterEnabledOnly']()}</option>
				<option value="disabled">{m['admin.profiles.filterDisabledOnly']()}</option>
			</select>
			<button
				class="rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted"
				onclick={onResetFilters}
			>
				{m['admin.profiles.resetFilters']()}
			</button>
		</div>
	{/if}

	<div class="overflow-x-auto rounded-lg border border-border">
		<table class="w-full text-sm">
			<thead class="bg-muted/50 text-left">
				<tr>
					<SortableTooltipHeader text={m['admin.profiles.id']()} tooltip={m['admin.profiles.tooltip.id']()} column="id" activeColumn={sortBy} {sortDir} onSort={handleSort} />
					<SortableTooltipHeader text={m['admin.profiles.labelField']()} tooltip={m['admin.profiles.tooltip.id']()} column="label" activeColumn={sortBy} {sortDir} onSort={handleSort} />
					<SortableTooltipHeader text={m['admin.profiles.vcpu']()} tooltip={m['admin.profiles.tooltip.vcpu']()} column="cpuCores" activeColumn={sortBy} {sortDir} onSort={handleSort} />
					<SortableTooltipHeader text={m['common.memory']()} tooltip={m['admin.profiles.tooltip.memory']()} column="memoryMB" activeColumn={sortBy} {sortDir} onSort={handleSort} />
					<SortableTooltipHeader text={m['admin.profiles.disk']()} tooltip={m['admin.profiles.tooltip.disk']()} column="diskGB" activeColumn={sortBy} {sortDir} onSort={handleSort} />
					<TooltipHeader text={m['admin.profiles.bus']()} tooltip={m['admin.profiles.tooltip.bus']()} />
					<th class="px-4 py-2 font-medium">{m['admin.profiles.enabledStatus']()}</th>
					<th class="px-4 py-2 font-medium">{m['common.actions']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each filteredProfiles as profile (profile.id)}
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
								<Button variant="secondary" size="sm" label={m['admin.profiles.editLabel']({ label: profile.label })} onclick={() => openEdit(profile)}>{m['admin.profiles.edit']()}</Button>
								<Button variant="destructive" size="sm" label={m['admin.profiles.deleteLabel']({ label: profile.label })} onclick={() => (pendingDelete = profile)}>{m['admin.profiles.delete']()}</Button>
							</div>
						</td>
					</tr>
				{:else}
					<tr><td colspan={8} class="p-0">
						{#if profiles.length > 0}
							<EmptyState title={m['admin.profiles.noFilterMatches']()} />
						{:else}
							<EmptyState title={m['admin.profiles.noProfiles']()}>
								{#snippet actions()}
									<Button onclick={openCreate}>{m['admin.profiles.newProfile']()}</Button>
								{/snippet}
							</EmptyState>
						{/if}
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
