<script lang="ts">
	import type { AdminDocPage, DocCreateInput, DocUpdateInput } from './docs.svelte';
	import DocsFormDialog from './DocsFormDialog.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Switch from '$lib/shared/ui/Switch.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import TableHeader from '$lib/shared/ui/TableHeader.svelte';
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

	function handleSort(column: string): void {
		onSort(column as DocSortColumn);
	}

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

	function confirmDelete(page: AdminDocPage): void {
		if (page.isSystem) return;
		if (window.confirm(m['docs.confirmDelete']())) {
			onDelete(page.id, page.lang);
		}
	}

	function audienceBadgeClass(a: 'user' | 'admin'): string {
		return a === 'admin'
			? 'bg-destructive/10 text-destructive'
			: 'bg-primary/10 text-primary';
	}
</script>

<svelte:head>
	<title>{m['docs.title']()} — PVMSS</title>
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
	<p role="alert" class="text-destructive">{error}</p>
{:else}
	{#if saveError}
		<p role="alert" class="mb-4 rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">
			{saveError}
		</p>
	{/if}

	{#if pages.length > 0}
		<div class="mb-4 flex flex-wrap items-center gap-2">
			<input
				type="search"
				class="rounded-md border border-border bg-background px-3 py-1.5 text-sm"
				placeholder={m['admin.docs.searchPlaceholder']()}
				value={search}
				oninput={(e) => onSearchChange(e.currentTarget.value)}
			/>
			<select class="rounded-md border border-border bg-background px-3 py-1.5 text-sm" value={categoryFilter} onchange={(e) => onCategoryFilterChange(e.currentTarget.value)}>
				<option value="">{m['admin.docs.filterCategory']()}</option>
				{#each categoryOptions as cat (cat)}
					<option value={cat}>{cat}</option>
				{/each}
			</select>
			<select class="rounded-md border border-border bg-background px-3 py-1.5 text-sm" value={langFilter} onchange={(e) => onLangFilterChange(e.currentTarget.value)}>
				<option value="">{m['admin.docs.filterLang']()}</option>
				{#each langOptions as lang (lang)}
					<option value={lang}>{lang}</option>
				{/each}
			</select>
			<select class="rounded-md border border-border bg-background px-3 py-1.5 text-sm" value={audienceFilter} onchange={(e) => onAudienceFilterChange(e.currentTarget.value as 'all' | 'user' | 'admin')}>
				<option value="all">{m['admin.docs.filterAudience']()}</option>
				<option value="user">{m['docs.audienceUser']()}</option>
				<option value="admin">{m['docs.audienceAdmin']()}</option>
			</select>
			<button
				class="rounded-md border border-border px-3 py-1.5 text-sm text-muted-foreground hover:bg-muted"
				onclick={onResetFilters}
			>
				{m['admin.docs.resetFilters']()}
			</button>
		</div>
	{/if}

	<div class="overflow-x-auto rounded-lg border border-border">
		<table class="w-full text-sm">
			<thead class="bg-muted/50 text-left">
				<tr>
					<TableHeader text={m['docs.titleField']()} tooltip={m['admin.docs.searchPlaceholder']()} column="title" activeColumn={sortBy} {sortDir} onSort={handleSort} />
					<TableHeader text={m['docs.category']()} tooltip={m['admin.docs.filterCategory']()} column="category" activeColumn={sortBy} {sortDir} onSort={handleSort} />
					<th class="px-4 py-2 font-medium">{m['docs.audience']()}</th>
					<TableHeader text={m['docs.language']()} tooltip={m['admin.docs.filterLang']()} column="lang" activeColumn={sortBy} {sortDir} onSort={handleSort} />
					<th class="px-4 py-2 font-medium">{m['docs.enabled']()}</th>
					<th class="px-4 py-2 font-medium">{m['admin.docs.actions']()}</th>
				</tr>
			</thead>
			<tbody>
				{#each filteredPages as page (`${page.id}-${page.lang}`)}
					<tr class="border-t border-border">
						<td class="px-4 py-2">
							<div class="flex flex-col">
								<span>{page.title}</span>
								<span class="font-mono text-xs text-muted-foreground">{page.id}</span>
							</div>
						</td>
						<td class="px-4 py-2">{page.category}</td>
						<td class="px-4 py-2">
							<span class={`inline-block rounded px-2 py-0.5 text-xs font-medium ${audienceBadgeClass(page.audience)}`}>
								{page.audience === 'admin' ? m['docs.audienceAdmin']() : m['docs.audienceUser']()}
							</span>
						</td>
						<td class="px-4 py-2 font-mono text-xs">{page.lang}</td>
						<td class="px-4 py-2">
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
						<td class="px-4 py-2">
							<div class="flex gap-2">
								<Button variant="secondary" size="sm" label={m['admin.docs.editLabel']({ title: page.title })} onclick={() => openEdit(page)}>{m['admin.docs.edit']()}</Button>
								<Button
									variant="destructive"
									size="sm"
									label={m['admin.docs.deleteLabel']({ title: page.title })}
									disabled={page.isSystem}
									onclick={() => confirmDelete(page)}
								>{m['admin.docs.delete']()}</Button>
							</div>
							{#if page.isSystem}
								<p class="mt-1 text-xs text-muted-foreground">{m['docs.systemProtected']()}</p>
							{/if}
						</td>
					</tr>
				{:else}
					<tr><td colspan={6} class="p-0">
						{#if pages.length > 0}
							<EmptyState title={m['admin.docs.noFilterMatches']()} />
						{:else}
							<EmptyState title={m['docs.empty']()}>
								{#snippet actions()}
									<Button onclick={openCreate}>{m['docs.newPage']()}</Button>
								{/snippet}
							</EmptyState>
						{/if}
					</td></tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

<DocsFormDialog
	{showForm}
	{editing}
	{title}
	{slug}
	{lang}
	{category}
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
