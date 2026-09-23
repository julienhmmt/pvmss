<script lang="ts">
	import { getAdminBaselineContext } from './baseline.svelte';
	import { fullyPublished } from '$lib/features/admin-cloudinit-templates/cloudInitTemplates.svelte';
	import PageHeader from '$lib/shared/ui/PageHeader.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import Pill from '$lib/shared/ui/Pill.svelte';
	import Skeleton from '$lib/shared/ui/Skeleton.svelte';
	import ErrorState from '$lib/shared/ui/ErrorState.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const store = getAdminBaselineContext();

	function refresh(): void {
		void store.load('default');
	}
</script>

<PageHeader
	title={m['admin.baseline.heading']()}
	description={m['admin.baseline.generatedHint']()}
	titleId="baseline-title"
>
	{#snippet actions()}
		<Button variant="secondary" size="sm" loading={store.loading} onclick={refresh}>
			{m['common.refresh']()}
		</Button>
	{/snippet}
</PageHeader>

<section class="space-y-8" aria-labelledby="baseline-title">
	{#if store.loading}
		<Card as="div" pad="md">
			<Skeleton class="h-4 w-32" />
			<Skeleton class="mt-2 h-64 w-full" />
		</Card>
	{:else if store.error}
		<ErrorState title={store.error} retry={refresh} retryLabel={m['common.retry']()} />
	{:else if store.state}
		<div role="status" aria-live="polite" class="sr-only">
			{m['admin.baseline.generated']()}
		</div>

		<p class="text-sm text-muted-foreground">{m['admin.baseline.createTimeOnly']()}</p>

		<!-- Generated baseline -->
		<div class="space-y-4">
			<h2 class="text-lg font-semibold" id="baseline-generated-heading">
				{m['admin.baseline.generated']()}
			</h2>
			<Card as="div" pad="md">
				<pre class="max-h-[480px] overflow-auto whitespace-pre-wrap break-words font-mono text-sm">{store.state.generated}</pre>
			</Card>
		</div>

		<!-- Publication state -->
		<div class="space-y-4">
			<h2 class="text-lg font-semibold" id="baseline-publication-heading">
				{m['admin.baseline.publication']()}
			</h2>
			<Card as="div" pad="md">
				{#if store.state.publication === null}
					<Pill tone="warn" label={m['admin.cloudinit.notPublished']()} />
					<p class="mt-2 text-sm text-muted-foreground">{m['admin.baseline.publishHint']()}</p>
				{:else}
					<Pill
						tone={fullyPublished(store.state.publication) ? 'ok' : 'warn'}
						label={m['admin.cloudinit.publishedNodes']({
							ok: store.state.publication.nodes.filter((n) => n.ok).length,
							total: store.state.publication.nodes.length
						})}
					/>
					<p class="mt-2 font-mono text-xs text-muted-foreground" data-testid="baseline-filename">{store.state.publication.filename}</p>
					{#each store.state.publication.nodes.filter((n) => !n.ok) as failed (failed.node)}
						<p class="mt-1 text-sm text-warning">{failed.node}: {failed.error}</p>
					{/each}
				{/if}
			</Card>
		</div>
	{:else}
		<p class="py-8 text-center text-muted-foreground">{m['admin.baseline.noCluster']()}</p>
	{/if}
</section>
