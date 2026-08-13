<script lang="ts">
	import Skeleton from './Skeleton.svelte';

	/**
	 * TableSkeleton — renders N skeleton rows × a configurable column count
	 * inside a full table shell (the same `overflow-x-auto rounded-lg border`
	 * wrapper the real admin tables use), so the table shape doesn't jump
	 * when data arrives. Replaces the bare "Loading…" paragraph on
	 * table-bearing admin pages. The pulse is zeroed by the global
	 * prefers-reduced-motion rule in app.css.
	 */
	interface Props {
		rows?: number;
		columns: number;
	}

	let { rows = 5, columns }: Props = $props();
</script>

<div class="overflow-x-auto rounded-lg border border-border" aria-hidden="true">
	<table class="w-full text-sm">
		<thead class="bg-muted/50 text-left">
			<tr>
				{#each Array(columns) as _, i (i)}
					<th class="px-4 py-2 font-medium"><Skeleton class="h-4 w-24" /></th>
				{/each}
			</tr>
		</thead>
		<tbody>
			{#each Array(rows) as _, i (i)}
				<tr class="border-t border-border">
					{#each Array(columns) as _, j (j)}
						<td class="px-4 py-2"><Skeleton class="h-4 w-full" /></td>
					{/each}
				</tr>
			{/each}
		</tbody>
	</table>
</div>
