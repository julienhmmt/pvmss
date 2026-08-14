<script lang="ts">
	import { getDbOpsContext } from './dbOps.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getDbOpsContext();
</script>

<section class="space-y-3">
	<h2 class="text-xl font-semibold tracking-tight">{m['admin.db.exportTitle']()}</h2>
	<p class="text-sm text-muted-foreground">
		{m['admin.db.exportDescription']()}
	</p>

	{#if store.exportError}
		<p role="alert" class="rounded-md bg-destructive/10 px-4 py-2 text-sm text-destructive">{store.exportError}</p>
	{/if}

	<Button disabled={store.exporting} onclick={() => void store.exportDatabase()}>
		{store.exporting ? m['common.exporting']() : m['admin.db.export']()}
	</Button>
</section>
