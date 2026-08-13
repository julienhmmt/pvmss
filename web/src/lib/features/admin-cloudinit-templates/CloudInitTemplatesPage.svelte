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
							<button
								type="button"
								class="rounded-md px-3 py-1 text-xs font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring {template.enabled
									? 'bg-primary text-primary-foreground'
									: 'bg-muted text-muted-foreground hover:bg-muted/80'}"
								onclick={() => onToggle(template.id, !template.enabled)}
							>
								{template.enabled ? 'Enabled' : 'Disabled'}
							</button>
						</td>
						<td class="px-4 py-2">
							<div class="flex gap-2">
								<button
									type="button"
									class="text-xs text-muted-foreground hover:text-foreground"
									onclick={() => openEdit(template)}
								>
									Edit
								</button>
								<button
									type="button"
									class="text-xs text-destructive hover:text-destructive/80"
									onclick={() => onDelete(template.id)}
								>
									Delete
								</button>
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
