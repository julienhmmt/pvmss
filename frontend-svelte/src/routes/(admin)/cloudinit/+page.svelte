<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { Badge } from '$lib/components/ui/badge';
	import * as Card from '$lib/components/ui/card';
	import * as Dialog from '$lib/components/ui/dialog';
	import * as Table from '$lib/components/ui/table';
	import {
		getCloudInits,
		createCloudInit,
		updateCloudInit,
		deleteCloudInit,
		toggleCloudInit
	} from '$lib/api/admin/cloudinit';
	import { Cloud } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { CloudInitTemplate, SFTPStatus } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let templates = $state<CloudInitTemplate[]>([]);
	let sftpStatus = $state<SFTPStatus | undefined>(undefined);
	let editOpen = $state(false);
	let editId = $state<string | null>(null);
	let deleteTarget = $state<string | null>(null);
	let form = $state({ name: '', description: '', storage: '', content: '' });

	let statusColorClass = $derived(
		sftpStatus?.status_type === 'success'
			? 'text-green-600 border-green-300'
			: sftpStatus?.status_type === 'warning'
				? 'text-yellow-600 border-yellow-300'
				: 'text-red-600 border-red-300'
	);

	let badgeVariant = $derived<'default' | 'secondary' | 'destructive' | 'outline'>(
		sftpStatus?.status_type === 'success'
			? 'default'
			: sftpStatus?.status_type === 'warning'
				? 'secondary'
				: 'destructive'
	);

	async function load() {
		loading = true;
		error = null;
		try {
			const data = await getCloudInits();
			templates = data.templates;
			sftpStatus = data.sftp_status;
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	function openCreate() {
		editId = null;
		form = { name: '', description: '', storage: '', content: '' };
		editOpen = true;
	}

	function openEdit(t: CloudInitTemplate) {
		editId = t.id;
		form = { name: t.name, description: t.description, storage: t.storage, content: '' };
		editOpen = true;
	}

	async function handleSave() {
		if (!form.name) return;
		try {
			if (editId) {
				await updateCloudInit(editId, form);
				toast.success('Template updated');
			} else {
				await createCloudInit(form);
				toast.success('Template created');
			}
			editOpen = false;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	async function handleDelete() {
		if (!deleteTarget) return;
		try {
			await deleteCloudInit(deleteTarget);
			toast.success('Template deleted');
			deleteTarget = null;
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	async function handleToggle(id: string) {
		try {
			await toggleCloudInit(id);
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	onMount(load);
</script>

<PageHeader title="Cloud-Init Templates" icon={Cloud}>
	{#snippet actions()}
		<Button onclick={openCreate}>Create Template</Button>
	{/snippet}
</PageHeader>

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else}
	{#if sftpStatus}
		<Card.Root class="mb-6 {statusColorClass}">
			<Card.Header>
				<Card.Title class="flex items-center gap-2">
					SFTP Status
					<Badge variant={badgeVariant}>{sftpStatus.status_text}</Badge>
				</Card.Title>
			</Card.Header>
			<Card.Content>
				<div class="grid grid-cols-2 gap-4 text-sm sm:grid-cols-4">
					<div>
						<span class="text-muted-foreground">Enabled</span>
						<p class="font-medium">{sftpStatus.enabled ? 'Yes' : 'No'}</p>
					</div>
					{#if sftpStatus.host}
						<div>
							<span class="text-muted-foreground">Host</span>
							<p class="font-medium">{sftpStatus.host}</p>
						</div>
					{/if}
					{#if sftpStatus.username}
						<div>
							<span class="text-muted-foreground">Username</span>
							<p class="font-medium">{sftpStatus.username}</p>
						</div>
					{/if}
					<div>
						<span class="text-muted-foreground">Key Exists</span>
						<p class="font-medium">{sftpStatus.key_exists ? 'Yes' : 'No'}</p>
					</div>
				</div>
			</Card.Content>
		</Card.Root>
	{/if}

	{#if templates.length === 0}
		<EmptyState title="No cloud-init templates" icon={Cloud} description="Create a template to automate VM initialization" />
	{:else}
		<div class="rounded-md border">
			<Table.Root>
				<Table.Header>
					<Table.Row>
						<Table.Head>Name</Table.Head>
						<Table.Head>Description</Table.Head>
						<Table.Head>Storage</Table.Head>
						<Table.Head>Enabled</Table.Head>
						<Table.Head>Actions</Table.Head>
					</Table.Row>
				</Table.Header>
				<Table.Body>
					{#each templates as t}
						<Table.Row>
							<Table.Cell class="font-medium">{t.name}</Table.Cell>
							<Table.Cell>{t.description}</Table.Cell>
							<Table.Cell>{t.storage}</Table.Cell>
							<Table.Cell>
								<Switch checked={t.enabled} onCheckedChange={() => handleToggle(t.id)} />
							</Table.Cell>
							<Table.Cell>
								<div class="flex gap-2">
									<Button variant="outline" size="sm" onclick={() => openEdit(t)}>Edit</Button>
									<Button variant="destructive" size="sm" onclick={() => (deleteTarget = t.id)}>
										Delete
									</Button>
								</div>
							</Table.Cell>
						</Table.Row>
					{/each}
				</Table.Body>
			</Table.Root>
		</div>
	{/if}
{/if}

<Dialog.Root bind:open={editOpen}>
	<Dialog.Content>
		<Dialog.Header>
			<Dialog.Title>{editId ? 'Edit Template' : 'Create Template'}</Dialog.Title>
		</Dialog.Header>
		<div class="space-y-4">
			<div class="space-y-2">
				<Label>Name</Label>
				<Input bind:value={form.name} placeholder="Template name" />
			</div>
			<div class="space-y-2">
				<Label>Description</Label>
				<Input bind:value={form.description} placeholder="Description" />
			</div>
			<div class="space-y-2">
				<Label>Storage</Label>
				<Input bind:value={form.storage} placeholder="local" />
			</div>
			<div class="space-y-2">
				<Label>YAML Content</Label>
				<textarea
					class="flex min-h-[120px] w-full rounded-md border border-input bg-background px-3 py-2 text-sm ring-offset-background placeholder:text-muted-foreground focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring"
					bind:value={form.content}
					placeholder="#cloud-config"
				></textarea>
			</div>
		</div>
		<Dialog.Footer>
			<Button variant="outline" onclick={() => (editOpen = false)}>Cancel</Button>
			<Button onclick={handleSave} disabled={!form.name}>{editId ? 'Update' : 'Create'}</Button>
		</Dialog.Footer>
	</Dialog.Content>
</Dialog.Root>

<ConfirmDialog
	open={deleteTarget !== null}
	title="Delete Template"
	description="Are you sure you want to delete this cloud-init template?"
	confirmLabel="Delete"
	variant="destructive"
	onConfirm={handleDelete}
	onCancel={() => (deleteTarget = null)}
/>
