<script lang="ts">
	import type { AdminTag } from './tags.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		tags: AdminTag[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		onCreate: (name: string, color: string) => void;
		onUpdateColor: (name: string, color: string) => void;
		onDelete: (name: string) => void;
	}

	let {
		tags,
		loading,
		error,
		saving,
		saveError,
		onCreate,
		onUpdateColor,
		onDelete
	}: Props = $props();

	let showForm = $state(false);
	let newName = $state('');
	let newColor = $state('#4f46e5');
	let editingTag = $state<string | null>(null);
	let editColor = $state('');

	function openCreate(): void {
		newName = '';
		newColor = '#4f46e5';
		showForm = true;
	}

	function submitCreate(): void {
		onCreate(newName, newColor);
		showForm = false;
	}

	function startEdit(tag: AdminTag): void {
		editingTag = tag.name;
		editColor = tag.color;
	}

	function saveEdit(): void {
		if (editingTag) {
			onUpdateColor(editingTag, editColor);
		}
		editingTag = null;
	}
</script>

<svelte:head>
	<title>{m['admin.tags.pageTitle']()}</title>
</svelte:head>

<PageHeader title={m['admin.tags.title']()}>
	{#snippet actions()}
		<Button onclick={openCreate}>{m['admin.tags.newTag']()}</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={4} />
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
					<th class="px-4 py-2 font-medium">{m['common.name']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.tags.color']()}</th>
					<th class="px-4 py-2 font-medium">{m['common.vms']()}</th>
					<th class="px-4 py-2 font-medium">{m['common.actions']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each tags as tag (tag.name)}
					<tr class="border-t border-border">
						<td class="px-4 py-2 font-mono">
							{tag.name}
							{#if tag.protected}
								<span class="ml-2 rounded bg-muted px-1.5 py-0.5 text-xs text-muted-foreground">{m['admin.tags.protected']()}</span>
							{/if}
						</td>
						<td class="px-4 py-2">
							{#if editingTag === tag.name}
								<div class="flex items-center gap-2">
									<input type="color" bind:value={editColor} class="h-8 w-12 rounded border" />
									<Button variant="ghost" size="sm" onclick={saveEdit}>{m['common.save']()}</Button>
									<Button variant="ghost" size="sm" onclick={() => (editingTag = null)}>{m['common.cancel']()}</Button>
								</div>
							{:else}
								<div class="flex items-center gap-2">
									<span class="inline-block h-4 w-4 rounded-full border" style="background: {tag.color}"></span>
									<span class="font-mono text-xs">{tag.color}</span>
									<Button variant="ghost" size="sm" label={m['admin.tags.editColorLabel']({ name: tag.name })} onclick={() => startEdit(tag)}>{m['common.edit']()}</Button>
								</div>
							{/if}
						</td>
						<td class="px-4 py-2">{tag.vmCount}</td>
						<td class="px-4 py-2">
							{#if !tag.protected}
								<Button variant="destructive" size="sm" label={m['admin.tags.deleteLabel']({ name: tag.name })} onclick={() => onDelete(tag.name)}>{m['common.delete']()}</Button>
							{:else}
								<span class="text-xs text-muted-foreground">—</span>
							{/if}
						</td>
					</tr>
				{:else}
					<tr><td colspan={4} class="p-0">
						<EmptyState title={m['admin.tags.noTags']()}>
							{#snippet actions()}
								<Button onclick={openCreate}>{m['admin.tags.newTag']()}</Button>
							{/snippet}
						</EmptyState>
					</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

{#if showForm}
	<div class="fixed inset-0 z-50 flex items-center justify-center bg-black/50" role="dialog" aria-modal="true">
		<div class="w-full max-w-sm rounded-lg bg-background p-6 shadow-lg">
			<h2 class="mb-4 text-lg font-medium">{m['admin.tags.newTagForm']()}</h2>
			<form onsubmit={(e) => { e.preventDefault(); submitCreate(); }} class="space-y-4">
				<div>
					<label for="tag-name" class="mb-1 block text-sm font-medium">{m['admin.tags.nameField']()}</label>
					<input
						id="tag-name"
						type="text"
						pattern={'[a-zA-Z0-9]{1,50}'}
						class="w-full rounded-md border bg-background px-3 py-2 text-sm"
						bind:value={newName}
						required
					/>
				</div>
				<div>
					<label for="tag-color" class="mb-1 block text-sm font-medium">{m['admin.tags.colorField']()}</label>
					<input type="color" id="tag-color" bind:value={newColor} class="h-10 w-full rounded border" />
				</div>
				<div class="flex justify-end gap-2 pt-2">
					<Button variant="ghost" onclick={() => (showForm = false)}>{m['common.cancel']()}</Button>
					<Button type="submit" disabled={saving}>
						{saving ? m['common.creating']() : m['common.create']()}
					</Button>
				</div>
			</form>
		</div>
	</div>
{/if}
