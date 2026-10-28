<script lang="ts">
	/**
	 * Pagination - client-side footer: page-size selector, "x-y of N" and
	 * prev/next. Parent owns `page` and `size` (bind both) and slices with
	 * paginate(). Page resets to 1 when `total` or the size changes; the
	 * chosen size is remembered per `name` in localStorage.
	 */
	import Button from './Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { PAGE_SIZES, savePageSize } from './paginate';

	interface Props {
		total: number;
		page: number;
		size: number;
		/** localStorage key suffix identifying this list. */
		name: string;
	}

	let { total, page = $bindable(1), size = $bindable(25), name }: Props = $props();

	const pages = $derived(size <= 0 ? 1 : Math.max(1, Math.ceil(total / size)));
	const from = $derived(total === 0 ? 0 : size <= 0 ? 1 : (page - 1) * size + 1);
	const to = $derived(size <= 0 ? total : Math.min(total, page * size));

	$effect(() => {
		void total;
		void size;
		page = 1;
	});

	function onSizeChange(event: Event): void {
		size = Number((event.currentTarget as HTMLSelectElement).value);
		savePageSize(name, size);
	}
</script>

{#if total > 10}
	<div
		class="flex flex-wrap items-center justify-between gap-3 border-t border-border px-4 py-3 text-sm text-muted-foreground"
	>
		<label class="flex items-center gap-2">
			<span>{m['common.pagination.perPage']()}</span>
			<select class="pv-input pv-select w-auto" value={String(size)} onchange={onSizeChange}>
				{#each PAGE_SIZES as option (option)}
					<option value={String(option)}>{option === 0 ? m['common.all']() : option}</option>
				{/each}
			</select>
		</label>
		<div class="flex items-center gap-3">
			<span aria-live="polite">{m['common.pagination.range']({ from, to, total })}</span>
			<Button variant="secondary" size="sm" disabled={page <= 1} onclick={() => (page -= 1)}>{m['common.previous']()}</Button>
			<Button variant="secondary" size="sm" disabled={page >= pages} onclick={() => (page += 1)}>{m['common.next']()}</Button>
		</div>
	</div>
{/if}
