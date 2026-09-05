<script lang="ts">
	import type { AdminProfile } from './profiles.svelte';
	import type { ClusterOption } from '$lib/shared/clusters';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import Dialog from '$lib/shared/ui/Dialog.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
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
	<Alert>{error}</Alert>
{:else}
	{#if saveError}
		<Alert class="mb-4">{saveError}</Alert>
	{/if}

	{#if profiles.length > 0}
		<TableCard>
			{#snippet toolbar()}
				<TextField
					type="search"
					placeholder={m['admin.profiles.searchPlaceholder']()}
					value={search}
					oninput={(e: Event & { currentTarget: HTMLInputElement }) => onSearchChange(e.currentTarget.value)}
					class="w-full sm:w-48"
				/>
				<Select
					value={busFilter}
					onchange={(e: Event & { currentTarget: HTMLSelectElement }) => onBusFilterChange(e.currentTarget.value)}
					options={[
						{ value: '', label: m['admin.profiles.filterBus']() },
						...busOptions.map((bus) => ({ value: bus, label: bus }))
					]}
					class="w-full sm:w-44"
				/>
				<Select
					value={enabledFilter}
					onchange={(e: Event & { currentTarget: HTMLSelectElement }) => onEnabledFilterChange(e.currentTarget.value as 'all' | 'enabled' | 'disabled')}
					options={[
						{ value: 'all', label: m['admin.profiles.filterEnabled']() },
						{ value: 'enabled', label: m['admin.profiles.filterEnabledOnly']() },
						{ value: 'disabled', label: m['admin.profiles.filterDisabledOnly']() }
					]}
					class="w-full sm:w-44"
				/>
				<Button
					variant="secondary"
					size="sm"
					onclick={onResetFilters}
				>
					{m['admin.profiles.resetFilters']()}
				</Button>
			{/snippet}
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['admin.profiles.title']()}</caption>
				<thead>
					<tr>
						<TableHeader text={m['admin.profiles.id']()} tooltip={m['admin.profiles.tooltip.id']()} column="id" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['admin.profiles.labelField']()} tooltip={m['admin.profiles.tooltip.id']()} column="label" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['admin.profiles.vcpu']()} tooltip={m['admin.profiles.tooltip.vcpu']()} column="cpuCores" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['common.memory']()} tooltip={m['admin.profiles.tooltip.memory']()} column="memoryMB" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['admin.profiles.disk']()} tooltip={m['admin.profiles.tooltip.disk']()} column="diskGB" activeColumn={sortBy} {sortDir} onSort={handleSort} />
						<TableHeader text={m['admin.profiles.bus']()} tooltip={m['admin.profiles.tooltip.bus']()} />
						<th class="font-medium">{m['admin.profiles.enabledStatus']()}</th>
						<th class="font-medium">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each filteredProfiles as profile (profile.id)}
						<tr class="group transition-colors hover:bg-muted/40">
							<td class="font-mono text-xs" data-label={m['admin.profiles.id']()}>{profile.id}</td>
							<td data-label={m['admin.profiles.labelField']()}>{profile.label}</td>
							<td data-label={m['admin.profiles.vcpu']()}>{profile.cpuCores}</td>
							<td data-label={m['common.memory']()}>{profile.memoryMB} MB</td>
							<td data-label={m['admin.profiles.disk']()}>{profile.diskGB} GB</td>
							<td data-label={m['admin.profiles.bus']()}>{profile.bus}</td>
							<td data-label={m['admin.profiles.enabledStatus']()}>
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
							<td data-label={m['common.actions']()}>
								<div class="flex gap-2">
									<Button variant="secondary" size="sm" label={m['admin.profiles.editLabel']({ label: profile.label })} onclick={() => openEdit(profile)}>{m['admin.profiles.edit']()}</Button>
									<Button variant="destructive" size="sm" label={m['admin.profiles.deleteLabel']({ label: profile.label })} onclick={() => (pendingDelete = profile)}>{m['admin.profiles.delete']()}</Button>
								</div>
							</td>
						</tr>
					{:else}
						<tr><td colspan={8} class="p-0">
							<EmptyState title={m['admin.profiles.noFilterMatches']()} />
						</td></tr>
					{/each}
				</tbody>
			</table>
		</TableCard>
	{:else}
		<EmptyState title={m['admin.profiles.noProfiles']()}>
			{#snippet actions()}
				<Button onclick={openCreate}>{m['admin.profiles.newProfile']()}</Button>
			{/snippet}
		</EmptyState>
	{/if}
{/if}

<Dialog bind:open={showForm} size="lg" labelledBy="profile-form-title" onClose={() => (showForm = false)}>
	<h2 id="profile-form-title" class="mb-4 text-lg font-medium">{editingId ? m['admin.profiles.editProfile']() : m['admin.profiles.newProfileForm']()}</h2>
	<form onsubmit={(e) => { e.preventDefault(); submitForm(); }} class="space-y-4">
		<FormField label={m['admin.profiles.labelField']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} bind:value={label} required />
			{/snippet}
		</FormField>
		<div class="grid grid-cols-2 gap-4">
			<FormField label={m['admin.profiles.vcpuCores']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" min={1} bind:value={cpuCores} required />
				{/snippet}
			</FormField>
			<FormField label={m['admin.profiles.memoryMb']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" min={128} bind:value={memoryMB} required />
				{/snippet}
			</FormField>
			<FormField label={m['admin.profiles.diskGb']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" min={1} bind:value={diskGB} required />
				{/snippet}
			</FormField>
			<FormField label={m['admin.profiles.busField']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<Select
						{id}
						{describedBy}
						{invalid}
						bind:value={bus}
						options={['scsi', 'virtio', 'sata', 'ide']}
						required
					/>
				{/snippet}
			</FormField>
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
