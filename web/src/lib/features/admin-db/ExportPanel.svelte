<script lang="ts">
	import { getDbOpsContext } from './dbOps.svelte';

	const store = getDbOpsContext();
</script>

<section class="space-y-3">
	<h2 class="text-xl font-semibold tracking-tight">Export Database</h2>
	<p class="text-sm text-muted-foreground">
		Download a consistent snapshot of the configuration database. The export does not interrupt concurrent reads or writes.
	</p>

	{#if store.exportError}
		<p role="alert" class="rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{store.exportError}</p>
	{/if}

	<button
		type="button"
		class="rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground disabled:opacity-50"
		disabled={store.exporting}
		onclick={() => void store.exportDatabase()}
	>
		{store.exporting ? 'Exporting…' : 'Export'}
	</button>
</section>
