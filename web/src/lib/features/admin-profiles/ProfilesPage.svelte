<script lang="ts">
	import {
		buildProfileDraftJson,
		parseProfileDraftJson,
		vcpuTotal,
		type AdminProfile,
		type ProfileDraft,
		type ProfileExportItem,
		type ProfileImportOutcome,
		type ProfileSortColumn
	} from './profiles.svelte';
	import ProfileImportDialog from './ProfileImportDialog.svelte';
	import type { ClusterOption } from '#lib/shared/clusters.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import PageHeader from '#lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '#lib/shared/ui/ClusterSelector.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Switch from '#lib/shared/ui/Switch.svelte';
	import TableCard from '#lib/shared/ui/TableCard.svelte';
	import Pagination from '#lib/shared/ui/Pagination.svelte';
	import { createPagination } from '#lib/shared/ui/list-pager.svelte.js';
	const pager = createPagination('profiles', () => filteredProfiles);
	import TableSkeleton from '#lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import Textarea from '#lib/shared/ui/Textarea.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import TableHeader from '#lib/shared/ui/TableHeader.svelte';
	import { m } from '#lib/paraglide/messages.js';

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
		onCreate: (label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string, sockets: number, description: string) => Promise<boolean>;
		onUpdate: (id: string, label: string, cpuCores: number, memoryMB: number, diskGB: number, bus: string, sockets: number, description: string, newId: string) => Promise<boolean>;
		onDelete: (id: string) => void;
		onToggle: (id: string, enabled: boolean) => void;
		onClearSaveError: () => void;
		selected: ReadonlySet<string>;
		onToggleSelect: (id: string) => void;
		onToggleSelectAll: () => void;
		canReorder: boolean;
		onMove: (id: string, delta: -1 | 1) => void;
		exportContent: () => string;
		onImport: (clusters: string[], overwrite: boolean, items: ProfileExportItem[]) => Promise<ProfileImportOutcome[] | null>;
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
		onToggle,
		onClearSaveError,
		selected,
		onToggleSelect,
		onToggleSelectAll,
		canReorder,
		onMove,
		exportContent,
		onImport
	}: Props = $props();

	let showForm = $state(false);
	let showImport = $state(false);
	let expert = $state(false);
	let expertJson = $state('');
	let expertError = $state<string | null>(null);

	function downloadExport(): void {
		const url = URL.createObjectURL(new Blob([exportContent()], { type: 'application/json' }));
		const a = document.createElement('a');
		a.href = url;
		a.download = `pvmss-profiles-${cluster || 'cluster'}.json`;
		a.click();
		setTimeout(() => URL.revokeObjectURL(url));
	}

	const allSelected = $derived(profiles.length > 0 && selected.size === profiles.length);
	let editingId = $state<string | null>(null);
	let pendingDelete = $state<AdminProfile | null>(null);
	let label = $state('');
	let profileId = $state('');
	let sockets = $state(1);
	let cpuCores = $state(1);
	let memoryMB = $state(2048);
	let diskGB = $state(20);
	let bus = $state('scsi');
	let description = $state('');

	function currentDraft(): ProfileDraft {
		return { label, sockets, cpuCores, memoryMB, diskGB, bus, description };
	}

	function applyDraft(draft: ProfileDraft): void {
		label = draft.label;
		sockets = draft.sockets;
		cpuCores = draft.cpuCores;
		memoryMB = draft.memoryMB;
		diskGB = draft.diskGB;
		bus = draft.bus;
		description = draft.description;
	}

	function openCreate(): void {
		editingId = null;
		label = '';
		sockets = 1;
		cpuCores = 1;
		memoryMB = 2048;
		diskGB = 20;
		bus = 'scsi';
		description = '';
		expert = false;
		expertJson = '';
		expertError = null;
		onClearSaveError();
		showForm = true;
	}

	function openEdit(profile: AdminProfile): void {
		editingId = profile.id;
		profileId = profile.id;
		applyDraft({
			label: profile.label,
			sockets: profile.sockets ?? 1,
			cpuCores: profile.cpuCores,
			memoryMB: profile.memoryMB,
			diskGB: profile.diskGB,
			bus: profile.bus,
			description: profile.description ?? ''
		});
		expert = false;
		expertJson = '';
		expertError = null;
		onClearSaveError();
		showForm = true;
	}

	function toggleExpert(): void {
		if (expert) {
			// Leaving expert mode is always allowed, even when the editor holds
			// invalid JSON. The JSON is only validated on submit; the form
			// values kept while the editor was open are restored as-is.
			expert = false;
			expertError = null;
			return;
		}
		expertJson = buildProfileDraftJson(currentDraft());
		expertError = null;
		expert = true;
	}

	function onExpertInput(e: Event & { currentTarget: HTMLTextAreaElement }): void {
		expertJson = e.currentTarget.value;
		expertError = null;
	}

	function closeForm(): void {
		showForm = false;
		onClearSaveError();
	}

	async function submitForm(): Promise<void> {
		if (expert) {
			try {
				applyDraft(parseProfileDraftJson(expertJson));
			} catch (err) {
				expertError = err instanceof Error ? err.message : String(err);
				return;
			}
		}
		expertError = null;
		// A failed save keeps the dialog open with the draft intact and the
		// store's saveError rendered inside it.
		const succeeded = editingId
			? await onUpdate(editingId, label, cpuCores, memoryMB, diskGB, bus, sockets, description, profileId.trim())
			: await onCreate(label, cpuCores, memoryMB, diskGB, bus, sockets, description);
		if (succeeded) showForm = false;
	}

	const filtersActive = $derived(
		search !== '' || busFilter !== '' || enabledFilter !== 'all' || sortBy !== 'position' || sortDir !== 'asc'
	);
</script>

<svelte:head>
	<title>{m['admin.profiles.pageTitle']()}</title>
</svelte:head>

<PageHeader title={m['admin.profiles.title']()} eyebrow={m['chrome.adminNav.provisioning']()}>
	{#snippet actions()}
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="profiles-cluster" />
		<Button variant="secondary" onclick={() => { onClearSaveError(); showImport = true; }}>{m['admin.profiles.import']()}</Button>
		<Button variant="secondary" disabled={profiles.length === 0} onclick={downloadExport}>
			{selected.size > 0 ? m['admin.profiles.exportSelected']({ count: selected.size }) : m['admin.profiles.exportAll']()}
		</Button>
		<Button onclick={openCreate}>{m['admin.profiles.newProfile']()}</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={10} />
{:else if error}
	<Alert>{error}</Alert>
{:else}
	{#if saveError && !showForm}
		<Alert class="mb-4">{saveError}</Alert>
	{/if}

	{#if profiles.length > 0}
		<TableCard>
			{#snippet toolbar()}
				<TextField
					type="search"
					placeholder={m['admin.profiles.searchPlaceholder']()}
					aria-label={m['admin.profiles.searchPlaceholder']()}
					value={search}
					oninput={(e: Event & { currentTarget: HTMLInputElement }) => onSearchChange(e.currentTarget.value)}
					class="w-full sm:w-48"
				/>
				<Select
					aria-label={m['admin.profiles.bus']()}
					value={busFilter}
					onchange={(e: Event & { currentTarget: HTMLSelectElement }) => onBusFilterChange(e.currentTarget.value)}
					options={[
						{ value: '', label: m['admin.profiles.filterBus']() },
						...busOptions.map((bus) => ({ value: bus, label: bus }))
					]}
					class="w-full sm:w-44"
				/>
				<Select
					aria-label={m['common.enabled']()}
					value={enabledFilter}
					onchange={(e: Event & { currentTarget: HTMLSelectElement }) => onEnabledFilterChange(e.currentTarget.value as 'all' | 'enabled' | 'disabled')}
					options={[
						{ value: 'all', label: m['admin.profiles.filterEnabled']() },
						{ value: 'enabled', label: m['admin.profiles.filterEnabledOnly']() },
						{ value: 'disabled', label: m['admin.profiles.filterDisabledOnly']() }
					]}
					class="w-full sm:w-44"
				/>
				<span class="ml-auto text-sm text-muted-foreground" data-testid="profiles-count">
					{m['admin.profiles.resultCount']({ count: filteredProfiles.length })}
				</span>
				{#if filtersActive}
					<Button
						variant="secondary"
						size="sm"
						onclick={onResetFilters}
					>
						{m['admin.profiles.resetFilters']()}
					</Button>
				{/if}
			{/snippet}
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['admin.profiles.title']()}</caption>
				<thead>
					<tr>
						<th class="w-8">
							<input type="checkbox" aria-label={m['admin.profiles.selectAll']()} checked={allSelected} onchange={onToggleSelectAll} />
						</th>
						<TableHeader text={m['admin.profiles.order']()} tooltip={m['admin.profiles.tooltip.order']()} column="position" activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['admin.profiles.id']()} tooltip={m['admin.profiles.tooltip.id']()} column="id" activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['admin.profiles.labelField']()} tooltip={m['admin.profiles.tooltip.id']()} column="label" activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['admin.profiles.vcpu']()} tooltip={m['admin.profiles.tooltip.vcpu']()} column="cpuCores" activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['common.memory']()} tooltip={m['admin.profiles.tooltip.memory']()} column="memoryMB" activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['admin.profiles.disk']()} tooltip={m['admin.profiles.tooltip.disk']()} column="diskGB" activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['admin.profiles.bus']()} tooltip={m['admin.profiles.tooltip.bus']()} />
						<th class="font-medium">{m['admin.profiles.enabledStatus']()}</th>
						<th class="font-medium">{m['common.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each pager.rows as profile (profile.id)}
						{@const index = profiles.findIndex((p) => p.id === profile.id)}
						<tr class="group transition-colors hover:bg-muted/40">
							<td data-label={m['admin.profiles.select']()}>
								<input type="checkbox" aria-label={m['admin.profiles.selectLabel']({ label: profile.label })} checked={selected.has(profile.id)} onchange={() => onToggleSelect(profile.id)} />
							</td>
							<td data-label={m['admin.profiles.order']()}>
								<span class="inline-flex items-center gap-1">
									<span class="w-5 text-xs text-muted-foreground tabular-nums">{index + 1}</span>
									{#if canReorder}
										<Button variant="secondary" size="sm" label={m['admin.profiles.moveUp']({ label: profile.label })} disabled={index === 0} onclick={() => onMove(profile.id, -1)}>↑</Button>
										<Button variant="secondary" size="sm" label={m['admin.profiles.moveDown']({ label: profile.label })} disabled={index === profiles.length - 1} onclick={() => onMove(profile.id, 1)}>↓</Button>
									{/if}
								</span>
							</td>
							<td class="font-mono text-xs" data-label={m['admin.profiles.id']()}>{profile.id}</td>
							<td data-label={m['admin.profiles.labelField']()}>
								{profile.label}
								{#if profile.description}
									<span class="block max-w-xs truncate text-xs text-muted-foreground" title={profile.description}>{profile.description}</span>
								{/if}
							</td>
							<td data-label={m['admin.profiles.vcpu']()}>{vcpuTotal(profile)}</td>
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
						<tr><td colspan={10} class="p-0">
							<EmptyState title={m['admin.profiles.noFilterMatches']()} />
						</td></tr>
					{/each}
				</tbody>
			</table>
			{#snippet footer()}
				<Pagination {pager} />
			{/snippet}
		</TableCard>
	{:else}
		<EmptyState title={m['admin.profiles.noProfiles']()}>
			{#snippet actions()}
				<Button onclick={openCreate}>{m['admin.profiles.newProfile']()}</Button>
			{/snippet}
		</EmptyState>
	{/if}
{/if}

<Dialog bind:open={showForm} size="lg" labelledBy="profile-form-title" onClose={closeForm}>
	<div class="mb-4 flex items-center justify-between gap-3">
		<h2 id="profile-form-title" class="text-lg font-medium">{editingId ? m['admin.profiles.editProfile']() : m['admin.profiles.newProfileForm']()}</h2>
		<Button variant="secondary" size="sm" onclick={toggleExpert}>
			{expert ? m['admin.profiles.simpleMode']() : m['admin.profiles.expertMode']()}
		</Button>
	</div>
	<form onsubmit={(e) => { e.preventDefault(); void submitForm(); }} class="space-y-4">
		{#if saveError}
			<Alert>{saveError}</Alert>
		{/if}
		{#if expert}
			<p class="text-sm text-muted-foreground">{m['admin.profiles.expertHint']()}</p>
			{#if expertError}
				<Alert>{expertError}</Alert>
			{/if}
			<label for="profile-expert-json" class="block text-sm font-medium">{m['admin.profiles.expertMode']()}</label>
			<textarea
				id="profile-expert-json"
				class="pv-input font-mono text-xs"
				rows="14"
				spellcheck="false"
				value={expertJson}
				oninput={onExpertInput}
				data-testid="profile-expert-json"
			></textarea>
		{:else}
		{#if editingId}
			<FormField label={m['admin.profiles.id']()} hint={m['admin.profiles.tooltip.id']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} bind:value={profileId} required pattern="[a-z0-9]+(-[a-z0-9]+)*" />
				{/snippet}
			</FormField>
		{/if}
		<FormField label={m['admin.profiles.labelField']()} required>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} bind:value={label} required />
			{/snippet}
		</FormField>
		<div class="grid grid-cols-2 gap-4">
			<FormField label={m['admin.profiles.sockets']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" min={1} bind:value={sockets} required />
				{/snippet}
			</FormField>
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
		<FormField label={m['admin.profiles.descriptionField']()} hint={m['admin.profiles.descriptionHint']()}>
			{#snippet children({ id, describedBy, invalid })}
				<Textarea {id} {describedBy} {invalid} bind:value={description} rows={3} />
			{/snippet}
		</FormField>
		{/if}
		<div class="flex justify-end gap-2 pt-2">
			<Button variant="secondary" onclick={closeForm}>{m['common.cancel']()}</Button>
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

<ProfileImportDialog
	bind:open={showImport}
	{clusterOptions}
	{cluster}
	{saving}
	{saveError}
	{onImport}
	onClose={() => (showImport = false)}
/>
