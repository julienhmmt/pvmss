<script lang="ts">
	import type { AdminCloudInitTemplate } from './cloudInitTemplates.svelte';
	import CloudInitTemplateFormDialog from './CloudInitTemplateFormDialog.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	interface Props {
		templates: AdminCloudInitTemplate[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		onCreate: (label: string, content: string) => void;
		onUpdate: (id: string, label: string, content: string) => void;
		onDelete: (id: string) => void;
		onToggle: (id: string, enabled: boolean) => void;
	}

	let {
		templates,
		loading,
		error,
		saving,
		saveError,
		onCreate,
		onUpdate,
		onDelete,
		onToggle
	}: Props = $props();

	let showForm = $state(false);
	let editingId = $state<string | null>(null);
	let label = $state('');
	let content = $state('#cloud-config\n');

	function openCreate(): void {
		editingId = null;
		label = '';
		content = '#cloud-config\n';
		showForm = true;
	}

	function openEdit(template: AdminCloudInitTemplate): void {
		editingId = template.id;
		label = template.label;
		content = template.content;
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
	<title>Admin Cloud-init Templates — PVMSS</title>
</svelte:head>

<PageHeader title="Cloud-init Templates">
	{#snippet actions()}
		<Button onclick={openCreate}>New template</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
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
					<th class="px-4 py-2 font-medium">ID</th>
					<th class="px-4 py-2 font-medium">Label</th>
					<th class="px-4 py-2 font-medium">Enabled</th>
					<th class="px-4 py-2 font-medium">Actions</th>
				</tr>
			</thead>
			<tbody>
				{#each templates as template (template.id)}
					<tr class="border-t border-border">
						<td class="px-4 py-2 font-mono text-xs">{template.id}</td>
						<td class="px-4 py-2">{template.label}</td>
						<td class="px-4 py-2">
							<Button
								variant={template.enabled ? 'primary' : 'secondary'}
								size="sm"
								label={template.enabled ? `Disable ${template.label}` : `Enable ${template.label}`}
								onclick={() => onToggle(template.id, !template.enabled)}
							>
								{template.enabled ? 'Enabled' : 'Disabled'}
							</Button>
						</td>
						<td class="px-4 py-2">
							<div class="flex gap-2">
								<Button variant="ghost" size="sm" label={`Edit ${template.label}`} onclick={() => openEdit(template)}>Edit</Button>
								<Button variant="destructive" size="sm" label={`Delete ${template.label}`} onclick={() => onDelete(template.id)}>Delete</Button>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<CloudInitTemplateFormDialog
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
