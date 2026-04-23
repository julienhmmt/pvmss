<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Select from '$lib/components/ui/select';
	import {
		getProfiles,
		createProfile,
		updateProfile,
		deleteProfile,
		toggleProfile
	} from '$lib/api/admin/profiles';
	import type { VMProfileConfig, VMCreateSettings } from "$lib/types/vm-create";
	import { PROFILE_COLOR_CLASSES } from "$lib/types/vm-create";
	import { getVMCreateSettings } from '$lib/api/vm-create';
	import {
		Globe,
		Code,
		Cube,
		Database,
		Flask,
		Monitor,
		Cpu,
		HardDrive,
		Cloud,
		Info,
		Trash,
		PencilSimple,
		Plus,
		CheckCircle,
		XCircle
	} from 'phosphor-svelte';

	// ── Icon registry ─────────────────────────────────────────────────────────

	const ICON_COMPONENTS: Record<string, typeof Globe> = {
		Globe, Code, Cube, Database, Flask, Monitor, Cpu, HardDrive, Cloud, Info
	};

	const ICON_OPTIONS = Object.keys(ICON_COMPONENTS);
	const COLOR_OPTIONS = Object.keys(PROFILE_COLOR_CLASSES);

	// ── State ─────────────────────────────────────────────────────────────────

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let profiles = $state<VMProfileConfig[]>([]);
	let vmSettings = $state<VMCreateSettings | null>(null);

	let editOpen = $state(false);
	let editId = $state<string | null>(null);
	let deleteTarget = $state<string | null>(null);
	let toggling = $state<string | null>(null);
	let saving = $state(false);

	const emptyForm = (): Omit<VMProfileConfig, 'id'> & { id: string } => ({
		id: '',
		name: '',
		description: '',
		sockets: 1,
		cores: 1,
		ramGb: 2,
		diskGb: 16,
		diskBus: 'virtio',
		node: '',
		storage: '',
		icon: 'Globe',
		color: 'blue',
		enabled: true,
		enableEfi: true
	});

	let form = $state(emptyForm());

	const deleteTargetName = $derived(
		profiles.find((p) => p.id === deleteTarget)?.name ?? deleteTarget ?? ''
	);

	// ── Data loading ──────────────────────────────────────────────────────────

	async function load() {
		if (profiles.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			const [profileData, settingsData] = await Promise.all([
				getProfiles(),
				getVMCreateSettings()
			]);
			profiles = profileData.profiles;
			vmSettings = settingsData;
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	// ── Dialog helpers ────────────────────────────────────────────────────────

	function openCreate() {
		editId = null;
		form = emptyForm();
		editOpen = true;
	}

	function openEdit(profile: VMProfileConfig) {
		editId = profile.id;
		form = { ...profile, node: profile.node ?? '', storage: profile.storage ?? '', enableEfi: profile.enableEfi ?? true };
		editOpen = true;
	}

	// ── CRUD handlers ─────────────────────────────────────────────────────────

	async function handleSave() {
		if (!form.name.trim()) return;
		if (form.sockets < 1 || form.cores < 1 || form.ramGb < 1 || form.diskGb < 1) {
			toast.error($t('admin.profiles.validation.numericRequired'));
			return;
		}
		saving = true;
		try {
			const { id, ...rest } = form;
			if (editId) {
				await updateProfile(editId, rest);
				toast.success($t('admin.profiles.toast.updated', { values: { name: form.name } }));
			} else {
				await createProfile({ ...rest, id: id || undefined });
				toast.success($t('admin.profiles.toast.created', { values: { name: form.name } }));
			}
			editOpen = false;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			saving = false;
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		const name = deleteTargetName;
		try {
			await deleteProfile(deleteTarget);
			toast.success($t('admin.profiles.toast.deleted', { values: { name } }));
			deleteTarget = null;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	async function handleToggle(profile: VMProfileConfig) {
		toggling = profile.id;
		try {
			await toggleProfile(profile.id);
			const key = profile.enabled ? 'disabled' : 'enabled';
			toast.success($t(`admin.profiles.toast.${key}`, { values: { name: profile.name } }));
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			toggling = null;
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.profiles.title')}</title>
</svelte:head>

<!-- Page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.profiles.title')}</h1>
			{#if !loading}
				<p class="pv-subtitle">{$t('admin.profiles.subtitle')}</p>
			{/if}
		</div>
		{#if !loading}
			<div class="flex items-center gap-3">
				{#if profiles.length > 0}
					<div class="pv-header-stats">
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('common.total')}</div>
							<div class="pv-header-stat-value">{profiles.length}</div>
						</div>
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('common.enabled')}</div>
							<div class="pv-header-stat-value">{profiles.filter((p) => p.enabled).length}</div>
						</div>
					</div>
				{/if}
				<Button class="pv-header-btn" variant="outline" onclick={openCreate}>
					<Plus class="mr-1.5 h-4 w-4" />
					{$t('admin.profiles.createProfile')}
				</Button>
			</div>
		{/if}
	</div>
</div>

<div class="pv-content-width">

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if profiles.length === 0}
	{#snippet createButton()}
		<Button onclick={openCreate}>
			<Plus class="mr-1.5 h-4 w-4" />
			{$t('admin.profiles.createProfile')}
		</Button>
	{/snippet}
	<EmptyState
		title={$t('admin.profiles.noProfiles')}
		description={$t('admin.profiles.noProfilesHint')}
		action={createButton}
	/>
{:else}
	<!-- Profile grid -->
	<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
		{#each profiles as profile (profile.id)}
			{@const ProfileIcon = ICON_COMPONENTS[profile.icon] ?? Globe}
			{@const colors = PROFILE_COLOR_CLASSES[profile.color] ?? PROFILE_COLOR_CLASSES['gray']}
			<div
				class="rounded-xl border bg-card p-5 shadow-sm transition-opacity {profile.enabled
					? ''
					: 'opacity-50'}"
			>
				<div class="flex items-start gap-4">
					<div class="flex-shrink-0 rounded-lg p-2.5 {colors.bg}">
						<ProfileIcon class="h-5 w-5 {colors.icon}" />
					</div>
					<div class="min-w-0 flex-1">
						<div class="flex items-center gap-2">
							<p class="text-sm font-semibold leading-tight">{profile.name}</p>
							{#if profile.enabled}
								<CheckCircle class="text-emerald-500 h-3.5 w-3.5 flex-shrink-0" />
							{:else}
								<XCircle class="text-muted-foreground h-3.5 w-3.5 flex-shrink-0" />
							{/if}
						</div>
						<p class="text-muted-foreground mt-0.5 text-xs leading-snug">{profile.description}</p>
						<div class="mt-2.5 flex flex-wrap gap-1.5">
							<span class="bg-muted rounded px-1.5 py-0.5 text-xs font-medium">
								{profile.sockets * profile.cores} vCPU
							</span>
							<span class="bg-muted rounded px-1.5 py-0.5 text-xs font-medium">
								{profile.ramGb} GB RAM
							</span>
							<span class="bg-muted rounded px-1.5 py-0.5 text-xs font-medium">
								{profile.diskGb} GB disk
							</span>
							<span class="bg-muted rounded px-1.5 py-0.5 text-xs font-medium">
								{profile.diskBus}
							</span>
							{#if profile.enableEfi}
								<span class="bg-blue-100 dark:bg-blue-900/30 text-blue-700 dark:text-blue-300 rounded px-1.5 py-0.5 text-xs font-medium">
									EFI
								</span>
							{/if}
							{#if profile.node}
								<span class="bg-muted rounded px-1.5 py-0.5 text-xs font-medium">
									{$t('admin.profiles.fields.node')}: {profile.node}
								</span>
							{/if}
							{#if profile.storage}
								<span class="bg-muted rounded px-1.5 py-0.5 text-xs font-medium">
									{$t('admin.profiles.fields.storage')}: {profile.storage}
								</span>
							{/if}
						</div>
					</div>
				</div>
				<div class="mt-4 flex items-center justify-between border-t pt-3">
					<Switch
						checked={profile.enabled}
						disabled={toggling === profile.id}
						onCheckedChange={() => handleToggle(profile)}
					/>
					<div class="flex gap-1.5">
						<Button size="sm" variant="ghost" onclick={() => openEdit(profile)}>
							<PencilSimple class="h-4 w-4" />
						</Button>
						<Button
							size="sm"
							variant="ghost"
							class="text-destructive hover:text-destructive"
							onclick={() => (deleteTarget = profile.id)}
						>
							<Trash class="h-4 w-4" />
						</Button>
					</div>
				</div>
			</div>
		{/each}
	</div>
{/if}

<!-- Edit / Create dialog -->
<Dialog.Root bind:open={editOpen}>
	<Dialog.Content class="max-w-lg">
		<Dialog.Header>
			<Dialog.Title>
				{editId ? $t('admin.profiles.editProfile') : $t('admin.profiles.createProfile')}
			</Dialog.Title>
		</Dialog.Header>

		<div class="grid gap-4 py-2">
			<!-- Name -->
			<div class="grid gap-1.5">
				<Label for="p-name">{$t('admin.profiles.fields.name')}</Label>
				<Input id="p-name" bind:value={form.name} placeholder="Web Server" />
			</div>

			<!-- Description -->
			<div class="grid gap-1.5">
				<Label for="p-desc">{$t('admin.profiles.fields.description')}</Label>
				<Input id="p-desc" bind:value={form.description} placeholder="Nginx, Apache…" />
			</div>

			<!-- Specs row -->
			<div class="grid grid-cols-2 gap-3">
				<div class="grid gap-1.5">
					<Label for="p-sockets">{$t('admin.profiles.fields.sockets')}</Label>
					<Input id="p-sockets" type="number" min="1" max="8" bind:value={form.sockets} />
				</div>
				<div class="grid gap-1.5">
					<Label for="p-cores">{$t('admin.profiles.fields.cores')}</Label>
					<Input id="p-cores" type="number" min="1" max="64" bind:value={form.cores} />
				</div>
				<div class="grid gap-1.5">
					<Label for="p-ram">{$t('admin.profiles.fields.ramGb')}</Label>
					<Input id="p-ram" type="number" min="1" bind:value={form.ramGb} />
				</div>
				<div class="grid gap-1.5">
					<Label for="p-disk">{$t('admin.profiles.fields.diskGb')}</Label>
					<Input id="p-disk" type="number" min="1" bind:value={form.diskGb} />
				</div>
				<div class="grid gap-1.5">
					<Label for="p-bus">{$t('admin.profiles.fields.diskBus')}</Label>
					<Select.Root
						type="single"
						bind:value={form.diskBus}
					>
						<Select.Trigger id="p-bus">
							{form.diskBus || 'Select bus...'}
						</Select.Trigger>
						<Select.Content>
							{#each ['virtio', 'scsi', 'sata', 'ide'] as bus}
								<Select.Item value={bus}>{bus}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>

			<!-- Node + Storage override -->
			{#if vmSettings}
				<div class="grid grid-cols-2 gap-3">
					<div class="grid gap-1.5">
						<Label for="p-node">{$t('admin.profiles.fields.node')}</Label>
						<Select.Root type="single" bind:value={form.node}>
							<Select.Trigger id="p-node">
								{form.node || $t('admin.profiles.autoSelect')}
							</Select.Trigger>
							<Select.Content>
								<Select.Item value="">{$t('admin.profiles.autoSelect')}</Select.Item>
								{#each vmSettings.nodes.filter((n) => !n.disabled) as node}
									<Select.Item value={node.name}>{node.name}</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
					<div class="grid gap-1.5">
						<Label for="p-storage">{$t('admin.profiles.fields.storage')}</Label>
						<Select.Root type="single" bind:value={form.storage}>
							<Select.Trigger id="p-storage">
								{form.storage || $t('admin.profiles.autoSelect')}
							</Select.Trigger>
							<Select.Content>
								<Select.Item value="">{$t('admin.profiles.autoSelect')}</Select.Item>
								{#each vmSettings.storages as storage}
									<Select.Item value={storage.name}>
										{storage.name}{storage.node === '' ? ' — shared' : ''}
									</Select.Item>
								{/each}
							</Select.Content>
						</Select.Root>
					</div>
				</div>
			{/if}

			<!-- EFI toggle -->
			<div class="flex items-center gap-3 rounded-md border p-3">
				<Switch bind:checked={form.enableEfi} />
				<div>
					<p class="text-sm font-medium">{$t('admin.profiles.fields.enableEfi')}</p>
					<p class="text-xs text-muted-foreground">{$t('admin.profiles.fields.enableEfiHint')}</p>
				</div>
			</div>

			<!-- Icon + Color -->
			<div class="grid grid-cols-2 gap-3">
				<div class="grid gap-1.5">
					<Label for="p-icon">{$t('admin.profiles.fields.icon')}</Label>
					<Select.Root
						type="single"
						bind:value={form.icon}
					>
						<Select.Trigger id="p-icon">
							{$t(`admin.profiles.iconOptions.${form.icon}`) || 'Select icon...'}
						</Select.Trigger>
						<Select.Content>
							{#each ICON_OPTIONS as iconKey}
								<Select.Item value={iconKey}>
									{$t(`admin.profiles.iconOptions.${iconKey}`)}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
				<div class="grid gap-1.5">
					<Label for="p-color">{$t('admin.profiles.fields.color')}</Label>
					<Select.Root
						type="single"
						bind:value={form.color}
					>
						<Select.Trigger id="p-color">
							{$t(`admin.profiles.colorOptions.${form.color}`) || 'Select color...'}
						</Select.Trigger>
						<Select.Content>
							{#each COLOR_OPTIONS as colorKey}
								<Select.Item value={colorKey}>
									{$t(`admin.profiles.colorOptions.${colorKey}`)}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>

			<!-- Preview -->
			{#if form.name}
				{@const PreviewIcon = ICON_COMPONENTS[form.icon] ?? Globe}
				{@const previewColors = PROFILE_COLOR_CLASSES[form.color] ?? PROFILE_COLOR_CLASSES['gray']}
				<div class="rounded-lg border bg-muted/30 p-3">
					<p class="text-muted-foreground mb-2 text-xs font-medium uppercase tracking-wide">Preview</p>
					<div class="flex items-center gap-3">
						<div class="rounded-lg p-2 {previewColors.bg}">
							<PreviewIcon class="h-5 w-5 {previewColors.icon}" />
						</div>
						<div>
							<p class="text-sm font-semibold">{form.name}</p>
							<p class="text-muted-foreground text-xs">{form.description}</p>
						</div>
					</div>
				</div>
			{/if}

			<!-- Enabled toggle -->
			<div class="flex items-center gap-3">
				<Switch bind:checked={form.enabled} id="p-enabled" />
				<Label for="p-enabled">{$t('admin.profiles.fields.enabled')}</Label>
			</div>
		</div>

		<Dialog.Footer>
			<Button variant="outline" onclick={() => (editOpen = false)}>{$t('common.cancel')}</Button>
			<Button onclick={handleSave} disabled={saving || !form.name.trim()}>
				{saving ? $t('common.saving') : $t('common.save')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<!-- Delete confirm -->
<ConfirmDialog
	open={deleteTarget !== null}
	title={$t('common.confirmDelete')}
	description={$t('common.confirmDeleteDescription', { values: { name: deleteTargetName } })}
	onConfirm={handleDelete}
	onCancel={() => (deleteTarget = null)}
/>

</div>
