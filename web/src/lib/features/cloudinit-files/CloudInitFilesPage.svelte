<script lang="ts">
	import type { CloudInitFile, CloudInitFileSummary } from './cloudInitFiles.svelte';
	import CloudInitFileFormDialog from './CloudInitFileFormDialog.svelte';
	import { resolve } from '$app/paths';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		files: CloudInitFileSummary[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		getFile: (id: string) => Promise<CloudInitFile | null>;
		onCreate: (label: string, content: string) => void;
		onUpdate: (id: string, label: string, content: string) => void;
		onDelete: (id: string) => void;
	}

	let {
		files,
		loading,
		error,
		saving,
		saveError,
		getFile,
		onCreate,
		onUpdate,
		onDelete
	}: Props = $props();

	let showForm = $state(false);
	let editingId = $state<string | null>(null);
	let label = $state('');
	let content = $state('#cloud-config\n');
	let pendingDelete = $state<CloudInitFileSummary | null>(null);

	function openCreate(): void {
		editingId = null;
		label = '';
		content = '#cloud-config\n';
		showForm = true;
	}

	// The list endpoint withholds content — pull the full document so the
	// dialog opens with what is actually stored.
	async function openEdit(file: CloudInitFileSummary): Promise<void> {
		const full = await getFile(file.id);
		if (!full) return;
		editingId = full.id;
		label = full.label;
		content = full.content;
		showForm = true;
	}

	function submitForm(): void {
		if (editingId) {
			onUpdate(editingId, label, content);
		} else {
			onCreate(label, content);
		}
		showForm = false;
	}
</script>

<svelte:head>
	<title>{m['cloudinit.files.title']()}</title>
</svelte:head>

<PageHeader title={m['cloudinit.files.heading']()} description={m['cloudinit.files.description']()}>
	{#snippet actions()}
		<Button onclick={openCreate}>{m['cloudinit.files.new']()}</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={3} />
{:else if error}
	<Alert>{error}</Alert>
{:else}
	{#if saveError}
		<Alert class="mb-4">{saveError}</Alert>
	{/if}

	<TableCard>
		<table class="pv-table pv-responsive-table">
			<caption class="sr-only">{m['cloudinit.files.heading']()}</caption>
			<thead>
				<tr>
					<th class="font-medium">{m['cloudinit.files.label']()}</th>
					<th class="font-medium">{m['admin.cloudinit.id']()}</th>
					<th class="font-medium">{m['cloudinit.files.updated']()}</th>
					<th class="font-medium">{m['common.actions']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each files as file (file.id)}
					<tr class="group transition-colors hover:bg-muted/40">
						<td data-label={m['cloudinit.files.label']()}>{file.label}</td>
						<td class="font-mono text-xs" data-label={m['admin.cloudinit.id']()}>{file.id}</td>
						<td class="text-xs text-muted-foreground" data-label={m['cloudinit.files.updated']()}>{file.updatedAt}</td>
						<td data-label={m['common.actions']()}>
							<div class="flex gap-2">
								<Button variant="ghost" size="sm" label={m['admin.cloudinit.editLabel']({ label: file.label })} onclick={() => void openEdit(file)}>{m['cloudinit.files.edit']()}</Button>
								<Button variant="destructive" size="sm" label={m['admin.cloudinit.deleteLabel']({ label: file.label })} onclick={() => (pendingDelete = file)}>{m['cloudinit.files.delete']()}</Button>
							</div>
						</td>
					</tr>
				{:else}
					<tr><td colspan={4} class="p-0">
						<EmptyState title={m['cloudinit.files.empty']()} dataTestid="cloudinit-files-empty">
							{#snippet actions()}
								<Button onclick={openCreate}>{m['cloudinit.files.new']()}</Button>
								<ButtonLink variant="ghost" href={resolve('/docs/[id]', { id: 'cloud-init-howto' })}>{m['chrome.header.docs']()}</ButtonLink>
							{/snippet}
						</EmptyState>
					</td></tr>
				{/each}
			</tbody>
		</table>
	</TableCard>
{/if}

<CloudInitFileFormDialog
	{showForm}
	{editingId}
	{label}
	{content}
	{saving}
	onLabelChange={(v) => (label = v)}
	onContentChange={(v) => (content = v)}
	onCancel={() => (showForm = false)}
	onSubmit={submitForm}
/>

<ConfirmDialog
	open={pendingDelete !== null}
	title={m['cloudinit.files.delete']()}
	message={m['cloudinit.files.deleteConfirm']({ label: pendingDelete?.label ?? '' })}
	confirmLabel={m['common.deletePermanently']()}
	cancelLabel={m['common.cancel']()}
	confirming={saving}
	testId="cloudinit-file-delete-confirm"
	onConfirm={() => {
		if (pendingDelete) onDelete(pendingDelete.id);
		pendingDelete = null;
	}}
	onClose={() => (pendingDelete = null)}
/>
