<script lang="ts">
	/**
	 * Pagination - client-side footer: page-size selector, "x-y of N" and
	 * prev/next. Pass the ListPager from createPagination() (list-pager.svelte.ts);
	 * it owns the page, size and reset behaviour.
	 */
	import Button from './Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { PAGE_SIZES } from './paginate';
	import type { ListPager } from './list-pager.svelte';

	interface Props {
		pager: Pick<ListPager<unknown>, 'total' | 'page' | 'size'>;
	}

	let { pager }: Props = $props();

	const pages = $derived(pager.size <= 0 ? 1 : Math.max(1, Math.ceil(pager.total / pager.size)));
	const from = $derived(pager.total === 0 ? 0 : pager.size <= 0 ? 1 : (pager.page - 1) * pager.size + 1);
	const to = $derived(pager.size <= 0 ? pager.total : Math.min(pager.total, pager.page * pager.size));

	function onSizeChange(event: Event): void {
		pager.size = Number((event.currentTarget as HTMLSelectElement).value);
	}
</script>

{#if pager.total > 10}
	<div
		class="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-3 text-sm text-muted-foreground"
	>
		<label class="flex items-center gap-2">
			<span>{m['common.pagination.perPage']()}</span>
			<select class="pv-input pv-select w-auto" value={String(pager.size)} onchange={onSizeChange}>
				{#each PAGE_SIZES as option (option)}
					<option value={String(option)}>{option === 0 ? m['common.all']() : option}</option>
				{/each}
			</select>
		</label>
		<div class="flex items-center gap-3">
			<span aria-live="polite">{m['common.pagination.range']({ from, to, total: pager.total })}</span>
			<Button variant="secondary" size="sm" disabled={pager.page <= 1} onclick={() => (pager.page -= 1)}>{m['common.previous']()}</Button>
			<Button variant="secondary" size="sm" disabled={pager.page >= pages} onclick={() => (pager.page += 1)}>{m['common.next']()}</Button>
		</div>
	</div>
{/if}
