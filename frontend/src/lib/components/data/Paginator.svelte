<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import * as Select from '$lib/components/ui/select';

	interface Props {
		total: number;
		page: number;
		perPage: number;
		perPageOptions?: number[];
		hideIfSinglePage?: boolean;
	}

	let {
		total,
		page = $bindable(),
		perPage = $bindable(),
		perPageOptions = [10, 25, 50, 100],
		hideIfSinglePage = true
	}: Props = $props();

	const safePerPage = $derived(Math.max(1, perPage));
	const totalPages = $derived(Math.max(1, Math.ceil(total / safePerPage)));
	const startIndex = $derived(total === 0 ? 0 : (page - 1) * safePerPage + 1);
	const endIndex = $derived(Math.min(page * safePerPage, total));
	const hasPrev = $derived(page > 1);
	const hasNext = $derived(page < totalPages);

	$effect(() => {
		if (page > totalPages) page = totalPages;
		if (page < 1) page = 1;
	});

	function goPrev(): void {
		if (hasPrev) page = page - 1;
	}

	function goNext(): void {
		if (hasNext) page = page + 1;
	}

	function onPerPageChange(value: string | undefined): void {
		if (!value) return;
		const n = Number(value);
		if (!Number.isFinite(n) || n <= 0) return;
		perPage = n;
		page = 1;
	}
</script>

{#if !hideIfSinglePage || total > (perPageOptions[0] ?? 0) || totalPages > 1}
	<div class="flex flex-wrap items-center justify-between gap-3 pt-4">
		<p class="text-sm text-muted-foreground">
			{$t('common.pagination.showing', {
				values: { start: startIndex, end: endIndex, total }
			})}
		</p>

		<div class="flex items-center gap-4">
			<Select.Root type="single" value={String(perPage)} onValueChange={onPerPageChange}>
				<Select.Trigger class="w-[110px] h-8 text-sm" aria-label={$t('common.pagination.itemsPerPage')}>
					{$t('common.pagination.perPage', { values: { count: perPage } })}
				</Select.Trigger>
				<Select.Content>
					{#each perPageOptions as option, i (i)}
						<Select.Item value={String(option)}>{option}</Select.Item>
					{/each}
				</Select.Content>
			</Select.Root>

			<div class="flex items-center gap-2">
				<Button variant="outline" size="sm" disabled={!hasPrev} onclick={goPrev} aria-label={$t('common.previous')}>
					{$t('common.previous')}
				</Button>
				<span class="text-sm text-muted-foreground tabular-nums">
					{$t('common.pageOf', { values: { page, total: totalPages } })}
				</span>
				<Button variant="outline" size="sm" disabled={!hasNext} onclick={goNext} aria-label={$t('common.next')}>
					{$t('common.next')}
				</Button>
			</div>
		</div>
	</div>
{/if}
