<script lang="ts">
	import type { AdminCloudInitTemplate } from './cloudInitTemplates.svelte';
	import type { ClusterOption } from '$lib/shared/clusters';
	import CloudInitTemplateFormDialog from './CloudInitTemplateFormDialog.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import ClusterSelector from '$lib/shared/ui/ClusterSelector.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		templates: AdminCloudInitTemplate[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		clusterOptions: ClusterOption[];
		cluster: string;
		onClusterChange: (value: string) => void;
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
		clusterOptions,
		cluster,
		onClusterChange,
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
	<title>{m['admin.cloudinit.pageTitle']()}</title>
</svelte:head>

<PageHeader title={m['admin.cloudinit.header']()}>
	{#snippet actions()}
		<ClusterSelector options={clusterOptions} value={cluster} onChange={onClusterChange} id="cloudinit-cluster" />
		<Button onclick={openCreate}>{m['admin.cloudinit.newTemplate']()}</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
	<TableSkeleton columns={4} />
{:else if error}
	<Alert>{error}</Alert>
{:else}
	{#if saveError}
		<Alert class="mb-4">{saveError}</Alert>
	{/if}

	<TableCard>
		<table class="pv-table pv-responsive-table">
			<caption class="sr-only">{m['admin.cloudinit.header']()}</caption>
			<thead>
				<tr>
					<th class="font-medium">{m['admin.cloudinit.id']()}</th>
					<th class="font-medium">{m['admin.cloudinit.labelField']()}</th>
					<th class="font-medium">{m['admin.cloudinit.enabledStatus']()}</th>
					<th class="font-medium">{m['common.actions']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each templates as template (template.id)}
					<tr class="group transition-colors hover:bg-muted/40">
						<td class="font-mono text-xs" data-label={m['admin.cloudinit.id']()}>{template.id}</td>
						<td data-label={m['admin.cloudinit.labelField']()}>{template.label}</td>
						<td data-label={m['admin.cloudinit.enabledStatus']()}>
							<span class="inline-flex items-center gap-2">
								<Switch
									checked={template.enabled}
									label={template.enabled ? m['admin.cloudinit.disable']({ label: template.label }) : m['admin.cloudinit.enable']({ label: template.label })}
									onToggle={() => onToggle(template.id, !template.enabled)}
								/>
								<span class="text-xs text-muted-foreground">
									{template.enabled ? m['admin.cloudinit.enabledStatus']() : m['admin.cloudinit.disabledStatus']()}
								</span>
							</span>
						</td>
						<td data-label={m['common.actions']()}>
							<div class="flex gap-2">
								<Button variant="ghost" size="sm" label={m['admin.cloudinit.editLabel']({ label: template.label })} onclick={() => openEdit(template)}>{m['admin.cloudinit.edit']()}</Button>
								<Button variant="destructive" size="sm" label={m['admin.cloudinit.deleteLabel']({ label: template.label })} onclick={() => onDelete(template.id)}>{m['admin.cloudinit.delete']()}</Button>
							</div>
						</td>
					</tr>
				{:else}
					<tr><td colspan={4} class="p-0">
						<EmptyState title={m['admin.cloudinit.noTemplates']()}>
							{#snippet actions()}
								<Button onclick={openCreate}>{m['admin.cloudinit.newTemplate']()}</Button>
							{/snippet}
						</EmptyState>
					</td></tr>
				{/each}
			</tbody>
		</table>
	</TableCard>
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
