<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { CaretDown, CaretRight, Pencil, Plus } from 'phosphor-svelte';
	import { Button } from '$lib/components/ui/button';
	import { upsertSettings, type OverviewSection } from '$lib/api/admin/settings-overview';
	import type { OverviewSection as OverviewSectionType } from '$lib/types/admin-settings';

	interface Props {
		sectionKey: string;
		section: OverviewSectionType;
		onRefresh: () => void;
	}

	let { sectionKey, section, onRefresh }: Props = $props();

	let expanded = $state(false);
	let editing = $state(false);
	let adding = $state(false);
	let editValue = $state('');

	async function handleUpsert(record: unknown) {
		try {
			await upsertSettings({ table: sectionKey, record });
			toast.success($t('admin.settings.overview.updateSuccess'));
			onRefresh();
			editing = false;
			adding = false;
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	function handleSave() {
		try {
			if (Array.isArray(section.data)) {
				// Parse as line-separated list
				const items = editValue.split('\n').map((s) => s.trim()).filter((s) => s);
				handleUpsert(items);
			} else {
				// Parse as JSON
				const record = JSON.parse(editValue);
				handleUpsert(record);
			}
		} catch (e) {
			toast.error('Invalid data format');
		}
	}
</script>

<div class="rounded-xl border border-border bg-card overflow-hidden">
	<!-- Header -->
	<button
		class="w-full flex items-center justify-between p-4 hover:bg-accent/50 transition-colors"
		onclick={() => (expanded = !expanded)}
	>
		<div class="flex items-center gap-3 flex-1">
			{#if expanded}
				<CaretDown class="h-4 w-4" />
			{:else}
				<CaretRight class="h-4 w-4" />
			{/if}
			<div class="flex-1">
				<h3 class="font-medium text-sm">{section.name}</h3>
				<p class="text-xs text-muted-foreground">
					{section.row_count} {section.row_count === 1 ? 'item' : 'items'}
				</p>
			</div>
		</div>
		<div class="flex items-center gap-2">
			{#if section.supports_add}
				<Button
					variant="ghost"
					size="sm"
					class="h-8 w-8 p-0"
					onclick={(e) => {
						e.stopPropagation();
						editValue = Array.isArray(section.data) ? '' : '{}';
						adding = true;
					}}
				>
					<Plus class="h-4 w-4" />
				</Button>
			{/if}
			{#if section.supports_edit}
				<Button
					variant="ghost"
					size="sm"
					class="h-8 w-8 p-0"
					onclick={(e) => {
						e.stopPropagation();
						editValue = Array.isArray(section.data)
							? section.data.join('\n')
							: JSON.stringify(section.data, null, 2);
						editing = true;
					}}
				>
					<Pencil class="h-4 w-4" />
				</Button>
			{/if}
		</div>
	</button>

	{#if expanded}
		<!-- Content -->
		<div class="border-t border-border p-4">
			{#if editing || adding}
				<!-- Edit/Add form -->
				<div class="space-y-3">
					{#if Array.isArray(section.data) || adding}
						<textarea
							class="w-full text-sm bg-muted p-3 rounded min-h-32"
							bind:value={editValue}
							placeholder="Enter items, one per line"
						></textarea>
					{:else}
						<textarea
							class="w-full text-sm bg-muted p-3 rounded min-h-32"
							bind:value={editValue}
							placeholder="Enter JSON data"
						></textarea>
					{/if}
					<div class="flex gap-2 justify-end">
						<Button variant="outline" size="sm" onclick={() => { editing = false; adding = false; }}>
							{$t('common.cancel')}
						</Button>
						<Button size="sm" onclick={handleSave}>
							{$t('common.save')}
						</Button>
					</div>
				</div>
			{:else if section.row_count === 0}
				<p class="text-sm text-muted-foreground text-center py-4">
					{$t('admin.settings.overview.noItems')}
				</p>
			{:else}
				<pre class="text-xs bg-muted p-3 rounded overflow-auto max-h-64">{JSON.stringify(section.data, null, 2)}</pre>
			{/if}
		</div>
	{/if}
</div>
