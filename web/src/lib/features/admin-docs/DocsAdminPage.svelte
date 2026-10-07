<script lang="ts">
	import type { AdminDocPage, DocCreateInput, DocUpdateInput } from './docs.svelte';
	import DocsFormDialog from './DocsFormDialog.svelte';
	import Alert from '$lib/shared/ui/Alert.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import ConfirmDialog from '$lib/shared/ui/ConfirmDialog.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import TableCard from '$lib/shared/ui/TableCard.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import { m } from '$lib/paraglide/messages.js';

	type DocSortColumn = 'title' | 'id' | 'category' | 'lang';

	interface Props {
		pages: AdminDocPage[];
		filteredPages: AdminDocPage[];
		loading: boolean;
		error: string | null;
		saving: boolean;
		saveError: string | null;
		search: string;
		categoryFilter: string;
		langFilter: string;
		audienceFilter: 'all' | 'user' | 'admin';
		categoryOptions: string[];
		langOptions: string[];
		sortBy: DocSortColumn;
		sortDir: 'asc' | 'desc';
		onSearchChange: (value: string) => void;
		onCategoryFilterChange: (value: string) => void;
		onLangFilterChange: (value: string) => void;
		onAudienceFilterChange: (value: 'all' | 'user' | 'admin') => void;
		onSort: (column: DocSortColumn) => void;
		onResetFilters: () => void;
		onCreate: (input: DocCreateInput) => Promise<AdminDocPage | null>;
		onUpdate: (id: string, lang: string, input: DocUpdateInput) => Promise<AdminDocPage | null>;
		onDelete: (id: string, lang: string) => void;
		onToggle: (id: string, lang: string, enabled: boolean) => void;
	}

	let {
		pages,
		filteredPages,
		loading,
		error,
		saving,
		saveError,
		search,
		categoryFilter,
		langFilter,
		audienceFilter,
		categoryOptions,
		langOptions,
		sortBy,
		sortDir,
		onSearchChange,
		onCategoryFilterChange,
		onLangFilterChange,
		onAudienceFilterChange,
		onSort,
		onResetFilters,
		onCreate,
		onUpdate,
		onDelete,
		onToggle
	}: Props = $props();

	let showForm = $state(false);
	let editing = $state<AdminDocPage | null>(null);
	let title = $state('');
	let slug = $state('');
	let slugTouched = $state(false);
	let lang = $state('en');
	let category = $state('');
	let audience = $state<'user' | 'admin'>('user');
	let enabled = $state(true);
	let bodyMd = $state('');

	const SLUG_PATTERN = /[^a-z0-9-]+/g;

	function deriveSlug(value: string): string {
		return value
			.toLowerCase()
			.trim()
			.replace(/\s+/g, '-')
			.replace(SLUG_PATTERN, '')
			.replace(/-+/g, '-')
			.replace(/^-|-$/g, '');
	}

	function openCreate(): void {
		editing = null;
		title = '';
		slug = '';
		slugTouched = false;
		lang = 'en';
		category = '';
		audience = 'user';
		enabled = true;
		bodyMd = '# New page\n\n';
		showForm = true;
	}

	function openEdit(page: AdminDocPage): void {
		editing = page;
		title = page.title;
		slug = page.id;
		slugTouched = true;
		lang = page.lang;
		category = page.category;
		audience = page.audience;
		enabled = page.enabled;
		bodyMd = page.bodyMd;
		showForm = true;
	}

	function closeForm(): void {
		showForm = false;
	}

	function buildCreateInput(): DocCreateInput {
		return { title, lang, category, bodyMd, audience };
	}

	function buildUpdateInput(): DocUpdateInput {
		return { title, lang, category, bodyMd, audience, enabled, sortOrder: editing?.sortOrder ?? 0 };
	}

	async function submitSave(): Promise<void> {
		try {
			if (editing) {
				await onUpdate(editing.id, editing.lang, buildUpdateInput());
			} else {
				await onCreate(buildCreateInput());
			}
			showForm = false;
		} catch {
			// saveError is set by the store; keep the form open so the user can retry.
		}
	}

	async function submitSaveAndView(): Promise<void> {
		if (!editing) return;
		try {
			const updated = await onUpdate(editing.id, editing.lang, buildUpdateInput());
			showForm = false;
			if (updated) {
				window.location.href = `/docs/${updated.id}`;
			}
		} catch {
			// saveError is set by the store; keep the form open so the user can retry.
		}
	}

	function handleTitleChange(value: string): void {
		title = value;
		if (!slugTouched && !editing) {
			slug = deriveSlug(value);
		}
	}

	function handleSlugChange(value: string): void {
		slugTouched = true;
		slug = value;
	}

	let pendingDelete = $state<AdminDocPage | null>(null);

	function confirmDelete(): void {
		if (pendingDelete === null || pendingDelete.isSystem) return;
		onDelete(pendingDelete.id, pendingDelete.lang);
		pendingDelete = null;
	}
</script>

<svelte:head>
	<title>{m['docs.title']()} - PVMSS</title>
</svelte:head>

<PageHeader title={m['docs.title']()}>
	{#snippet actions()}
		<Button onclick={openCreate}>{m['docs.newPage']()}</Button>
	{/snippet}
</PageHeader>

{#if loading}
	<div role="status" aria-live="polite" class="sr-only">{m['docs.loading']()}</div>
	<TableSkeleton columns={6} />
{:else if error}
	<Alert>{error}</Alert>
{:else}
	{#if saveError}
		<Alert class="mb-4">{saveError}</Alert>
	{/if}

	{#if pages.length > 0}
		<TableCard>
			{#snippet toolbar()}
				<TextField
					type="search"
					class="w-full sm:w-64"
					placeholder={m['admin.docs.searchPlaceholder']()}
					value={search}
					oninput={(event: Event) => onSearchChange((event.currentTarget as HTMLInputElement).value)}
				/>
				<Select
					class="w-full sm:w-44"
					value={categoryFilter}
					onchange={(event: Event) => onCategoryFilterChange((event.currentTarget as HTMLSelectElement).value)}
					options={[{ value: '', label: m['admin.docs.filterCategory']() }, ...categoryOptions]}
				/>
				<Select
					class="w-full sm:w-44"
					value={langFilter}
					onchange={(event: Event) => onLangFilterChange((event.currentTarget as HTMLSelectElement).value)}
					options={[{ value: '', label: m['admin.docs.filterLang']() }, ...langOptions]}
				/>
				<Select
					class="w-full sm:w-44"
					value={audienceFilter}
					onchange={(event: Event) => onAudienceFilterChange((event.currentTarget as HTMLSelectElement).value as 'all' | 'user' | 'admin')}
					options={[
						{ value: 'all', label: m['admin.docs.filterAudience']() },
						{ value: 'user', label: m['docs.audienceUser']() },
						{ value: 'admin', label: m['docs.audienceAdmin']() }
					]}
				/>
				<Button variant="ghost" size="sm" onclick={onResetFilters}>
					{m['admin.docs.resetFilters']()}
				</Button>
			{/snippet}
			<table class="pv-table pv-responsive-table">
				<caption class="sr-only">{m['docs.title']()}</caption>
				<thead>
					<tr>
						<TableHeader text={m['docs.titleField']()} tooltip={m['admin.docs.searchPlaceholder']()} column="title" activeColumn={sortBy} {sortDir} {onSort} />
						<TableHeader text={m['docs.category']()} tooltip={m['admin.docs.filterCategory']()} column="category" activeColumn={sortBy} {sortDir} {onSort} />
						<th class="font-medium">{m['docs.audience']()}</th>
						<TableHeader text={m['docs.language']()} tooltip={m['admin.docs.filterLang']()} column="lang" activeColumn={sortBy} {sortDir} {onSort} />
						<th class="font-medium">{m['docs.enabled']()}</th>
						<th class="font-medium">{m['admin.docs.actions']()}</th>
					</tr>
				</thead>
				<tbody>
					{#each filteredPages as page (`${page.id}-${page.lang}`)}
						<tr class="group transition-colors hover:bg-muted/40">
							<td data-label={m['docs.titleField']()}>
								<div class="flex flex-col">
									<span>{page.title}</span>
									<span class="font-mono text-xs text-muted-foreground">{page.id}</span>
								</div>
							</td>
							<td data-label={m['docs.category']()}>{page.category}</td>
							<td data-label={m['docs.audience']()}>
								<Pill
									tone={page.audience === 'admin' ? 'error' : 'accent'}
									dot={false}
									label={page.audience === 'admin' ? m['docs.audienceAdmin']() : m['docs.audienceUser']()}
								/>
							</td>
							<td class="font-mono text-xs" data-label={m['docs.language']()}>{page.lang}</td>
							<td data-label={m['docs.enabled']()}>
								<span class="inline-flex items-center gap-2">
									<Switch
										checked={page.enabled}
										label={page.enabled ? m['admin.docs.disableLabel']({ title: page.title }) : m['admin.docs.enableLabel']({ title: page.title })}
										onToggle={() => onToggle(page.id, page.lang, !page.enabled)}
									/>
									<span class="text-xs text-muted-foreground">
										{page.enabled ? m['docs.enabled']() : m['admin.docs.disabled']()}
									</span>
								</span>
							</td>
							<td data-label={m['admin.docs.actions']()}>
								<div class="flex gap-2">
									<Button variant="secondary" size="sm" label={m['admin.docs.editLabel']({ title: page.title })} onclick={() => openEdit(page)}>{m['admin.docs.edit']()}</Button>
									<Button
										variant="destructive"
										size="sm"
										label={m['admin.docs.deleteLabel']({ title: page.title })}
										disabled={page.isSystem}
										onclick={() => (pendingDelete = page)}
									>{m['admin.docs.delete']()}</Button>
								</div>
								{#if page.isSystem}
									<p class="mt-1 text-xs text-muted-foreground">{m['docs.systemProtected']()}</p>
								{/if}
							</td>
						</tr>
					{:else}
						<tr><td colspan={6} class="p-0">
							<EmptyState title={m['admin.docs.noFilterMatches']()} />
						</td></tr>
					{/each}
				</tbody>
			</table>
		</TableCard>
	{:else}
		<EmptyState title={m['docs.empty']()}>
			{#snippet actions()}
				<Button onclick={openCreate}>{m['docs.newPage']()}</Button>
			{/snippet}
		</EmptyState>
	{/if}
{/if}

<DocsFormDialog
	{showForm}
	{editing}
	{title}
	{slug}
	{lang}
	{category}
	{categoryOptions}
	{audience}
	{enabled}
	{bodyMd}
	{saving}
	onTitleChange={handleTitleChange}
	onSlugChange={handleSlugChange}
	onLangChange={(v) => (lang = v)}
	onCategoryChange={(v) => (category = v)}
	onAudienceChange={(v) => (audience = v)}
	onEnabledChange={(v) => (enabled = v)}
	onBodyChange={(v) => (bodyMd = v)}
	onCancel={closeForm}
	onSave={submitSave}
	onSaveAndView={submitSaveAndView}
/>

<ConfirmDialog
	open={pendingDelete !== null}
	title={m['admin.docs.delete']()}
	message={m['docs.confirmDelete']()}
	confirmLabel={m['common.deletePermanently']()}
	cancelLabel={m['common.cancel']()}
	testId="doc-delete-confirm"
	onConfirm={confirmDelete}
	onClose={() => (pendingDelete = null)}
/>
