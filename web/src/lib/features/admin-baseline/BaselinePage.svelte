<script lang="ts">
	import { getAdminBaselineContext } from './baseline.svelte';
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

		<!-- Override state -->
		<div class="space-y-4">
			<h2 class="text-lg font-semibold" id="baseline-override-heading">
				{m['admin.baseline.override']({ filename: store.state.overrideFilename })}
			</h2>
			<Card as="div" pad="md">
				{#if store.state.overrideError && !store.state.overridePresent}
					<Pill tone="warn" label={m['admin.baseline.overrideError']({ error: store.state.overrideError })} />
				{:else if store.state.overridePresent}
					<Pill tone="ok" label={m['admin.baseline.overridePresent']()} />
					{#if store.state.overrideError}
						<p class="mt-2 text-sm text-muted-foreground">
							{m['admin.baseline.overrideError']({ error: store.state.overrideError })}
						</p>
					{/if}
					{#if store.state.overrideContent}
						<pre class="mt-4 max-h-[480px] overflow-auto whitespace-pre-wrap break-words font-mono text-sm">{store.state.overrideContent}</pre>
					{/if}
				{:else}
					<Pill tone="off" label={m['admin.baseline.overrideAbsent']()} />
				{/if}
			</Card>
		</div>
	{:else}
		<p class="py-8 text-center text-muted-foreground">{m['admin.baseline.noCluster']()}</p>
	{/if}
</section>
