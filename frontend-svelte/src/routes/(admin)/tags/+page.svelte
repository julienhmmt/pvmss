<script lang="ts">
	import { onMount } from 'svelte';
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
	import { Tag, X } from 'phosphor-svelte';
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
			toast.success(`Tag "${newTagName.trim()}" created`);
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
			toast.success(`Tag "${deleteTarget}" deleted`);
			deleteTarget = null;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	onMount(load);
</script>

<PageHeader title="Tags" icon={Tag}>
	{#snippet actions()}
		<Button onclick={() => (createOpen = true)}>Add Tag</Button>
	{/snippet}
</PageHeader>

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="card" rows={2} />
{:else if tags.length === 0}
	<EmptyState title="No tags configured" icon={Tag} description="Add tags to organize VMs" />
{:else}
	<div class="flex flex-wrap gap-2">
		{#each tags as tag}
			<Badge variant="secondary" class="gap-2 px-3 py-1.5 text-sm">
				{tag.name}
				<span class="text-muted-foreground">({tag.vm_count} VMs)</span>
				<button class="ml-1 hover:text-destructive" onclick={() => (deleteTarget = tag.name)}>
					<X class="h-3 w-3" />
				</button>
			</Badge>
		{/each}
	</div>
{/if}

<Dialog.Root bind:open={createOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>Create Tag</Dialog.Title>
			<Dialog.Description>Add a new tag for VM organization.</Dialog.Description>
		</Dialog.Header>
		<Input bind:value={newTagName} placeholder="Tag name" onkeydown={(e) => e.key === 'Enter' && handleCreate()} />
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (createOpen = false)}>Cancel</Button>
			<Button onclick={handleCreate} disabled={!newTagName.trim()}>Create</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<ConfirmDialog
	open={deleteTarget !== null}
	title="Delete Tag"
	description={`Are you sure you want to delete the tag "${deleteTarget}"?`}
	confirmLabel="Delete"
	variant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deleteTarget = null)}
/>
