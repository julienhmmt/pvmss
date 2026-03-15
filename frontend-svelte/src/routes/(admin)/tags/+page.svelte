<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Badge } from '$lib/components/ui/badge';
	import { Input } from '$lib/components/ui/input';
	import * as Dialog from '$lib/components/ui/dialog';
	import { getTags, createTag, deleteTag } from '$lib/api/admin/tags';
	import { Tag, X, Lock } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Tag as TagType } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let tags = $state<TagType[]>([]);
	let createOpen = $state(false);
	let newTagName = $state('');
	let deleteTarget = $state<string | null>(null);

	async function load() {
		loading = true;
		error = null;
		try {
			tags = await getTags();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
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

<PageHeader title={$t('admin.tags.title')} icon={Tag}>
	{#snippet actions()}
		<Button onclick={() => (createOpen = true)}>{$t('admin.tags.addTag')}</Button>
	{/snippet}
</PageHeader>

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={2} />
{:else if tags.length === 0}
	<EmptyState title={$t('admin.tags.noTags')} icon={Tag} description={$t('admin.tags.noTagsDesc')} />
{:else}
	<div class="flex flex-wrap gap-2">
		{#each tags as tag}
			<Badge variant="secondary" class="gap-2 px-3 py-1.5 text-sm">
				{tag.name}
				<span class="text-muted-foreground">({tag.vm_count} {$t('admin.tags.vms')})</span>
				{#if tag.name === 'pvmss'}
					<Lock class="ml-1 h-3 w-3 text-muted-foreground" />
				{:else}
					<button class="ml-1 hover:text-destructive" onclick={() => (deleteTarget = tag.name)}>
						<X class="h-3 w-3" />
					</button>
				{/if}
			</Badge>
		{/each}
	</div>
{/if}

<Dialog.Root bind:open={createOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>{$t('admin.tags.createTitle')}</Dialog.Title>
			<Dialog.Description>{$t('admin.tags.createDesc')}</Dialog.Description>
		</Dialog.Header>
		<Input bind:value={newTagName} placeholder={$t('admin.tags.namePlaceholder')} onkeydown={(e) => e.key === 'Enter' && handleCreate()} />
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
