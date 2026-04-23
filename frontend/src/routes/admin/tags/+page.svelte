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
	import * as Dialog from '$lib/components/ui/dialog';
	import { getTags, createTag, deleteTag } from '$lib/api/admin/tags';
	import { TagIcon, TrashIcon, LockIcon, MagnifyingGlass, PaletteIcon } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Tag as TagType } from '$lib/types/admin';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let tags = $state<TagType[]>([]);
	let createOpen = $state(false);
	let newTagName = $state('');
	let deleteTarget = $state<string | null>(null);
	let searchQuery = $state('');

	const filteredTags = $derived(
		searchQuery
			? tags.filter((t) => t.name.toLowerCase().includes(searchQuery.toLowerCase()))
			: tags
	);
	const proxmoxColoredCount = $derived(tags.filter((t) => t.fromProxmox).length);
	const totalVmCount = $derived(tags.reduce((acc, t) => acc + t.vmCount, 0));

	let page = $state(1);
	let perPage = $state(25);
	const pagedTags = $derived(paginate(filteredTags, page, perPage));

	$effect(() => {
		searchQuery;
		page = 1;
	});

	// Deterministic pastel palette for tags without a Proxmox-defined color.
	const fallbackPalette: ReadonlyArray<{ bg: string; fg: string }> = [
		{ bg: '#fef3c7', fg: '#92400e' },
		{ bg: '#dbeafe', fg: '#1e40af' },
		{ bg: '#dcfce7', fg: '#166534' },
		{ bg: '#fce7f3', fg: '#9d174d' },
		{ bg: '#e0e7ff', fg: '#3730a3' },
		{ bg: '#ffedd5', fg: '#9a3412' },
		{ bg: '#cffafe', fg: '#155e75' },
		{ bg: '#f3e8ff', fg: '#6b21a8' },
		{ bg: '#fee2e2', fg: '#991b1b' },
		{ bg: '#ecfccb', fg: '#3f6212' }
	];

	function hashString(value: string): number {
		let hash = 0;
		for (let i = 0; i < value.length; i += 1) {
			hash = (hash * 31 + value.charCodeAt(i)) | 0;
		}
		return Math.abs(hash);
	}

	function tagColors(tag: TagType): { bg: string; fg: string; fromProxmox: boolean } {
		if (tag.color) {
			return {
				bg: '#' + tag.color,
				fg: tag.textColor ? '#' + tag.textColor : contrastText('#' + tag.color),
				fromProxmox: true
			};
		}
		const palette = fallbackPalette[hashString(tag.name) % fallbackPalette.length];
		return { bg: palette.bg, fg: palette.fg, fromProxmox: false };
	}

	function contrastText(hex: string): string {
		const value = hex.replace('#', '');
		if (value.length !== 6) return '#111827';
		const r = parseInt(value.slice(0, 2), 16);
		const g = parseInt(value.slice(2, 4), 16);
		const b = parseInt(value.slice(4, 6), 16);
		const luminance = (0.299 * r + 0.587 * g + 0.114 * b) / 255;
		return luminance > 0.6 ? '#111827' : '#ffffff';
	}

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
							<div class="pv-header-stat-label">{$t('common.total')}</div>
							<div class="pv-header-stat-value">{tags.length}</div>
						</div>
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('admin.tags.color')}</div>
							<div class="pv-header-stat-value">{proxmoxColoredCount}</div>
						</div>
						<div class="pv-header-stat">
							<div class="pv-header-stat-label">{$t('admin.tags.vmCountLabel')}</div>
							<div class="pv-header-stat-value">{totalVmCount}</div>
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
	<div class="mb-4 flex flex-wrap items-center gap-3">
		<div class="relative flex-1 min-w-[200px] max-w-[360px]">
			<MagnifyingGlass class="absolute left-3 top-1/2 h-4 w-4 -translate-y-1/2 text-muted-foreground" />
			<input
				type="text"
				class="w-full rounded-md border border-border bg-background py-2 pl-9 pr-3 text-sm focus:outline-none focus:ring-2 focus:ring-ring"
				placeholder={$t('admin.tags.searchPlaceholder')}
				bind:value={searchQuery}
			/>
		</div>
		{#if searchQuery}
			<button
				type="button"
				class="text-xs text-muted-foreground hover:text-foreground underline"
				onclick={() => (searchQuery = '')}
			>
				{$t('common.clear')}
			</button>
		{/if}
	</div>

	{#if filteredTags.length === 0}
		<div class="pv-table-wrap py-12 text-center text-muted-foreground">
			<MagnifyingGlass class="mx-auto mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('admin.tags.noSearchResults')}</p>
		</div>
	{:else}
		<div class="pv-table-wrap">
			<table class="pv-table pv-tags-table">
				<thead>
					<tr>
						<th>{$t('admin.tags.preview')}</th>
						<th>{$t('admin.tags.tagName')}</th>
						<th>{$t('admin.tags.color')}</th>
						<th class="pv-th-num">{$t('admin.tags.vmCountLabel')}</th>
						<th class="pv-td-actions">{$t('common.actions')}</th>
					</tr>
				</thead>
				<tbody>
					{#each pagedTags as tag (tag.name)}
						{@const colors = tagColors(tag)}
						<tr class="pv-row">
							<td>
								<span
									class="pv-tag-chip"
									style="background:{colors.bg};color:{colors.fg};border-color:{colors.bg}"
								>
									<TagIcon class="h-3 w-3" weight="fill" />
									{tag.name}
								</span>
							</td>
							<td>
								<div class="flex items-center gap-2">
									<span class="pv-td-mono font-semibold">{tag.name}</span>
									{#if tag.name === 'pvmss'}
										<span class="pv-tag-protected" title={$t('admin.tags.protected')}>
											<LockIcon class="h-3 w-3" weight="fill" />
											{$t('admin.tags.protected')}
										</span>
									{/if}
								</div>
							</td>
							<td>
								<div
									class="flex items-center gap-2"
									title={colors.fromProxmox
										? $t('admin.tags.colorProxmoxHint')
										: $t('admin.tags.colorAutoHint')}
								>
									<span class="pv-tag-swatch" style="background:{colors.bg}"></span>
									<span class="pv-td-mono text-xs">{colors.bg.toUpperCase()}</span>
									{#if !colors.fromProxmox}
										<PaletteIcon class="h-3 w-3 text-muted-foreground" />
									{/if}
								</div>
							</td>
							<td class="pv-td-num">
								<span class="pv-action-badge pv-action-badge--vm">{tag.vmCount}</span>
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

		<Paginator total={filteredTags.length} bind:page bind:perPage />
	{/if}
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

<style>
	:global(.pv-tag-chip) {
		display: inline-flex;
		align-items: center;
		gap: 6px;
		padding: 4px 12px;
		border-radius: 99px;
		font-size: 0.8rem;
		font-weight: 600;
		letter-spacing: 0.01em;
		border: 1px solid transparent;
		white-space: nowrap;
		box-shadow: 0 1px 2px rgba(0, 0, 0, 0.06);
		transition: transform 0.15s ease;
	}
	:global(.pv-tag-chip:hover) {
		transform: translateY(-1px);
	}

	:global(.pv-tag-swatch) {
		display: inline-block;
		width: 18px;
		height: 18px;
		border-radius: 4px;
		border: 1px solid hsl(var(--border));
		flex-shrink: 0;
	}

	:global(.pv-tag-protected) {
		display: inline-flex;
		align-items: center;
		gap: 4px;
		padding: 2px 8px;
		border-radius: 6px;
		font-size: 0.68rem;
		font-weight: 600;
		text-transform: uppercase;
		letter-spacing: 0.05em;
		background: hsl(var(--muted));
		color: hsl(var(--muted-foreground));
		border: 1px solid hsl(var(--border));
	}
</style>
