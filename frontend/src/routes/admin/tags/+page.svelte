<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import * as Dialog from '$lib/components/ui/dialog';
	import { getTags, createTag, deleteTag } from '$lib/api/admin/tags';
	import { TagIcon, TrashIcon, LockIcon } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Tag as TagType } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let tags = $state<TagType[]>([]);
	let createOpen = $state(false);
	let newTagName = $state('');
	let deleteTarget = $state<string | null>(null);

	async function load() {
		if (tags.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			tags = await getTags();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function handleCreate() {
		if (!newTagName.trim()) return;
		try {
			await createTag(newTagName.trim());
			toast.success($t('admin.tags.toast.created', { values: { tagName: newTagName.trim() } }));
			newTagName = '';
			createOpen = false;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		try {
			await deleteTag(deleteTarget);
			toast.success($t('admin.tags.toast.deleted', { values: { tagName: deleteTarget } }));
			deleteTarget = null;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	onMount(load);
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.tags.title')}</title>
</svelte:head>

<!-- Gradient page header -->
<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.tags.title')}</h1>
			{#if !loading}
				<p class="pv-subtitle">
					{tags.length}
					{$t('admin.tags.title').toLowerCase()}
				</p>
			{/if}
		</div>

		{#if !loading}
			<div class="flex items-center gap-3">
				{#if tags.length > 0}
					<div class="pv-header-stats">
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('admin.tags.title')}</div>
							<div class="pv-header-stat-value">{tags.length}</div>
						</div>
					</div>
				{/if}
				<Button class="pv-header-btn" variant="outline" onclick={() => (createOpen = true)}>
					{$t('admin.tags.addTag')}
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
{:else if tags.length === 0}
	<EmptyState
		title={$t('admin.tags.noTags')}
		icon={TagIcon}
		description={$t('admin.tags.noTagsDesc')}
	/>
{:else}
	<div class="pv-table-wrap">
		<table class="pv-table">
			<thead>
				<tr>
					<th>{$t('admin.tags.tagName')}</th>
					<th class="pv-th-num">{$t('admin.tags.vmCountLabel')}</th>
					<th class="pv-td-actions">{$t('common.actions')}</th>
				</tr>
			</thead>
			<tbody>
				{#each tags as tag}
					<tr class="pv-row">
						<td>
							<div class="pv-resource-cell">
								<div class="pv-resource-icon" style="width:28px;height:28px;font-size:0.65rem">
									<TagIcon class="h-3.5 w-3.5" />
								</div>
								<span class="pv-td-mono">{tag.name}</span>
								{#if tag.name === 'pvmss'}
									<LockIcon class="h-3 w-3 text-muted-foreground" />
								{/if}
							</div>
						</td>
						<td class="pv-td-num">
							<span class="pv-action-badge pv-action-badge--vm">{tag.vm_count}</span>
						</td>
						<td class="pv-td-actions">
							{#if tag.name !== 'pvmss'}
								<Button
									variant="ghost"
									size="sm"
									class="text-destructive hover:text-destructive hover:bg-destructive/10"
									onclick={() => (deleteTarget = tag.name)}
								>
									<TrashIcon class="h-4 w-4" />
								</Button>
							{/if}
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<!-- Create tag dialog -->
<Dialog.Root bind:open={createOpen}>
	<Dialog.Content class="sm:max-w-md">
		<Dialog.Header>
			<Dialog.Title>{$t('admin.tags.createTitle')}</Dialog.Title>
			<Dialog.Description>{$t('admin.tags.createDesc')}</Dialog.Description>
		</Dialog.Header>
		<div class="py-2">
			<Input
				bind:value={newTagName}
				placeholder={$t('admin.tags.namePlaceholder')}
				onkeydown={(e) => e.key === 'Enter' && handleCreate()}
			/>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (createOpen = false)}>{$t('common.cancel')}</Button>
			<Button onclick={handleCreate} disabled={!newTagName.trim()}>{$t('common.create')}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<ConfirmDialog
	open={deleteTarget !== null}
	title={$t('admin.tags.deleteTitle')}
	description={$t('admin.tags.deleteDesc', { values: { tagName: deleteTarget } })}
	confirmLabel={$t('common.delete')}
	variant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deleteTarget = null)}
/>

</div>
