<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import Paginator from '$lib/components/data/Paginator.svelte';
	import { paginate } from '$lib/utils/paginate';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Select from '$lib/components/ui/select';
	import {
		getCloudInits,
		createCloudInit,
		updateCloudInit,
		deleteCloudInit,
		toggleCloudInit,
		toggleSFTP,
		getCloudInitStorages
	} from '$lib/api/admin/cloudinit';
	import { CloudIcon, TrashIcon, PencilIcon, CheckCircleIcon, WarningCircleIcon, XCircleIcon } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { CloudInitTemplate, SFTPStatus } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let templates = $state<CloudInitTemplate[]>([]);
	let sftpStatus = $state<SFTPStatus | undefined>(undefined);
	let snippetStorages = $state<string[]>([]);
	let editOpen = $state(false);
	let editId = $state<string | null>(null);
	let deleteTarget = $state<string | null>(null);
	let toggling = $state<string | null>(null);
	let togglingsftp = $state(false);
	let saving = $state(false);
	let form = $state({ name: '', description: '', storage: '', yamlContent: '' });

	let page = $state(1);
	let perPage = $state(25);
	const pagedTemplates = $derived(paginate(templates, page, perPage));

	const deleteTargetName = $derived(
		templates.find((t) => t.id === deleteTarget)?.name ?? deleteTarget ?? ''
	);

	async function load() {
		if (templates.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			const [data, storages] = await Promise.all([getCloudInits(), getCloudInitStorages()]);
			templates = data.templates;
			sftpStatus = data.sftpStatus;
			snippetStorages = storages;
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	function openCreate() {
		editId = null;
		form = { name: '', description: '', storage: '', yamlContent: '' };
		editOpen = true;
	}

	function openEdit(tmpl: CloudInitTemplate) {
		editId = tmpl.id;
		form = {
			name: tmpl.name,
			description: tmpl.description,
			storage: tmpl.storage,
			yamlContent: tmpl.yamlContent ?? ''
		};
		editOpen = true;
	}

	async function handleSave() {
		if (!form.name || !form.yamlContent) return;
		saving = true;
		try {
			if (editId) {
				await updateCloudInit(editId, form);
				toast.success($t('admin.cloudinit.toast.updated', { values: { name: form.name } }));
			} else {
				await createCloudInit(form);
				toast.success($t('admin.cloudinit.toast.created', { values: { name: form.name } }));
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
			await deleteCloudInit(deleteTarget);
			toast.success($t('admin.cloudinit.toast.deleted', { values: { name } }));
			deleteTarget = null;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	async function handleToggleSFTP() {
		togglingsftp = true;
		try {
			await toggleSFTP();
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			togglingsftp = false;
		}
	}

	async function handleToggle(tmpl: CloudInitTemplate) {
		toggling = tmpl.id;
		try {
			await toggleCloudInit(tmpl.id);
			if (tmpl.enabled) {
				toast.success($t('admin.cloudinit.toast.disabled', { values: { name: tmpl.name } }));
			} else {
				toast.success($t('admin.cloudinit.toast.enabled', { values: { name: tmpl.name } }));
			}
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
	<title>PVMSS — {$t('admin.cloudinit.title')}</title>
</svelte:head>

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.cloudinit.title')}</h1>
			{#if !loading}
				<p class="pv-subtitle">
					{templates.length}
					{$t('admin.cloudinit.templateCount', { values: { count: templates.length } })}
				</p>
			{/if}
		</div>

		{#if !loading}
			<div class="flex items-center gap-3">
				{#if templates.length > 0}
					<div class="pv-header-stats">
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('common.total')}</div>
							<div class="pv-header-stat-value">{templates.length}</div>
						</div>
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('common.enabled')}</div>
							<div class="pv-header-stat-value">{templates.filter((t) => t.enabled).length}</div>
						</div>
					</div>
				{/if}
				<Button class="pv-header-btn" variant="outline" onclick={openCreate}>
					{$t('admin.cloudinit.createTemplate')}
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
{:else}
	<!-- SFTP status panel -->
	{#if sftpStatus}
		{@const isSuccess = sftpStatus.statusType === 'success'}
		{@const isWarning = sftpStatus.statusType === 'warning'}
		<div class="mb-6 rounded-lg border p-4 {isSuccess ? 'border-success-soft-border bg-success-soft dark:border-green-900 dark:bg-green-950/30' : isWarning ? 'border-warning-soft-border bg-warning-soft dark:border-yellow-900 dark:bg-yellow-950/30' : 'border-destructive-soft-border bg-destructive-soft dark:border-red-900 dark:bg-red-950/30'}">
			<div class="flex items-start gap-3">
				<div class="mt-0.5 shrink-0">
					{#if isSuccess}
						<CheckCircleIcon class="h-5 w-5 text-success" />
					{:else if isWarning}
						<WarningCircleIcon class="h-5 w-5 text-warning-soft-foreground" />
					{:else}
						<XCircleIcon class="h-5 w-5 text-destructive" />
					{/if}
				</div>
				<div class="flex-1 min-w-0">
					<div class="flex items-center justify-between gap-3 flex-wrap">
						<p class="text-sm font-medium {isSuccess ? 'text-success-soft-foreground' : isWarning ? 'text-yellow-800 dark:text-yellow-300' : 'text-destructive-soft-foreground'}">
							{$t('admin.cloudinit.sftpStatus')} — {$t('admin.cloudinit.sftpStatusText.' + sftpStatus.statusText)}
						</p>
						{#if sftpStatus.isConfigured}
							<Button
								size="sm"
								variant="outline"
								disabled={togglingsftp}
								onclick={handleToggleSFTP}
								class="shrink-0 text-xs"
							>
								{sftpStatus.enabled ? $t('admin.cloudinit.sftpDisable') : $t('admin.cloudinit.sftpEnable')}
							</Button>
						{/if}
					</div>
					{#if sftpStatus.host || sftpStatus.username}
						<div class="mt-2 flex flex-wrap gap-4 text-xs text-muted-foreground">
							{#if sftpStatus.host}
								<span><span class="font-medium">{$t('admin.cloudinit.host')}:</span> {sftpStatus.host}</span>
							{/if}
							{#if sftpStatus.username}
								<span><span class="font-medium">{$t('admin.cloudinit.username')}:</span> {sftpStatus.username}</span>
							{/if}
							<span><span class="font-medium">{$t('admin.cloudinit.keyExists')}:</span> {sftpStatus.keyExists ? $t('common.yes') : $t('common.no')}</span>
						</div>
					{/if}
				</div>
			</div>
		</div>
	{/if}

	{#if templates.length === 0}
		<EmptyState
			title={$t('admin.cloudinit.noTemplates')}
			icon={CloudIcon}
			description={$t('admin.cloudinit.noTemplatesDesc')}
		/>
	{:else}
		<div class="pv-table-wrap">
			<table class="pv-table">
				<thead>
					<tr>
						<th>{$t('admin.cloudinit.templateName')}</th>
						<th>{$t('common.description')}</th>
						<th>{$t('common.storage')}</th>
						<th class="pv-td-actions">{$t('common.enabled')}</th>
						<th class="pv-td-actions">{$t('common.actions')}</th>
					</tr>
				</thead>
				<tbody>
					{#each pagedTemplates as tmpl (tmpl.id)}
						<tr class="pv-row" class:opacity-50={toggling === tmpl.id}>
							<td>
								<div class="pv-resource-cell">
									<div class="pv-resource-icon" style="width:28px;height:28px">
										<CloudIcon class="h-3.5 w-3.5" />
									</div>
									<span class="pv-td-mono">{tmpl.name}</span>
								</div>
							</td>
							<td class="pv-td-muted">{tmpl.description || '—'}</td>
							<td class="pv-td-muted">{tmpl.storage || '—'}</td>
							<td class="pv-td-actions">
								<Switch
									checked={tmpl.enabled}
									disabled={toggling === tmpl.id}
									onCheckedChange={() => handleToggle(tmpl)}
								/>
							</td>
							<td class="pv-td-actions">
								<div class="flex items-center gap-1">
									<Button
										variant="ghost"
										size="sm"
										onclick={() => openEdit(tmpl)}
									>
										<PencilIcon class="h-4 w-4" />
									</Button>
									<Button
										variant="ghost"
										size="sm"
										class="text-destructive hover:text-destructive hover:bg-destructive/10"
										onclick={() => (deleteTarget = tmpl.id)}
									>
										<TrashIcon class="h-4 w-4" />
									</Button>
								</div>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>

		<Paginator total={templates.length} bind:page bind:perPage />
	{/if}
{/if}

<!-- Create / Edit dialog -->
<Dialog.Root bind:open={editOpen}>
	<Dialog.Content class="sm:max-w-2xl">
		<Dialog.Header>
			<Dialog.Title>
				{editId ? $t('admin.cloudinit.editTemplate') : $t('admin.cloudinit.createTemplate')}
			</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4 py-2">
			<div class="space-y-2">
				<Label>{$t('admin.cloudinit.templateName')}</Label>
				<Input
					bind:value={form.name}
					placeholder={$t('admin.cloudinit.namePlaceholder')}
					disabled={!!editId}
				/>
			</div>
			<div class="space-y-2">
				<Label>{$t('common.description')}</Label>
				<Input bind:value={form.description} placeholder={$t('admin.cloudinit.descPlaceholder')} />
			</div>
			<div class="space-y-2">
				<Label>{$t('common.storage')}</Label>
				{#if snippetStorages.length > 0}
					<Select.Root
						type="single"
						value={form.storage}
						onValueChange={(v) => { form.storage = v ?? ''; }}
					>
						<Select.Trigger class="w-full">
							{form.storage || $t('admin.cloudinit.selectStorage')}
						</Select.Trigger>
						<Select.Content>
							{#each snippetStorages as s}
								<Select.Item value={s}>{s}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				{:else}
					<Input bind:value={form.storage} placeholder="local" />
					<p class="text-xs text-muted-foreground">{$t('admin.cloudinit.noSnippetStorages')}</p>
				{/if}
			</div>
			<div class="space-y-2">
				<Label>{$t('admin.cloudinit.yamlContent')}</Label>
				<textarea
					class="flex min-h-[200px] w-full rounded-md border border-input bg-background px-3 py-2 font-mono text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
					bind:value={form.yamlContent}
					placeholder="#cloud-config&#10;users:&#10;  - name: user"
				></textarea>
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (editOpen = false)}>{$t('common.cancel')}</Button>
			<Button
				onclick={handleSave}
				disabled={!form.name || !form.yamlContent || saving}
			>
				{saving ? $t('common.saving') : editId ? $t('common.update') : $t('common.create')}
			</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<ConfirmDialog
	open={deleteTarget !== null}
	title={$t('admin.cloudinit.deleteTitle')}
	description={$t('admin.cloudinit.deleteDesc', { values: { name: deleteTargetName } })}
	confirmLabel={$t('common.delete')}
	variant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deleteTarget = null)}
/>

</div>
