<script lang="ts">
	import type { Snippet } from 'svelte';
	import { getStatusContext } from '$lib/features/chrome/status.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { post } from '$lib/shared/api/client';

	/**
	 * ClusterDownOverlay renders a full-screen, non-interactive empty state
	 * when every configured cluster is unreachable. It wraps cluster-dependent
	 * pages so the user sees one consistent "PVMSS cannot work normally" message
	 * and cannot use search, filters, or create/edit actions until a cluster
	 * comes back. Pages that do not need a cluster (profile, docs) are not
	 * wrapped.
	 */
	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	const status = getStatusContext();

	async function retry(): Promise<void> {
		try {
			await post('/api/v1/cluster/refresh');
		} catch {
			// Cluster may still be down; re-poll will surface the current state.
		}
		await status.pollOnce();
	}
</script>

{#if status.allClustersDown}
	<div class="relative min-h-[50vh]">
		<div
			class="absolute inset-0 z-10 flex flex-col items-center justify-center gap-3 bg-background/95 px-4 py-12 text-center"
			data-testid="cluster-down-overlay"
		>
			<div class="max-w-md">
				<p class="text-lg font-semibold">{m['cluster.down.title']()}</p>
				<p class="mt-1 text-sm text-muted-foreground">{m['cluster.down.description']()}</p>
				<button
					type="button"
					class="mt-4 rounded-md bg-primary px-4 py-2 text-sm font-medium text-primary-foreground transition-colors hover:bg-primary/90 focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2"
					onclick={() => void retry()}
					data-testid="cluster-down-retry"
				>
					{m['cluster.down.retry']()}
				</button>
			</div>
		</div>
		<div
			class="pointer-events-none select-none opacity-40"
			aria-hidden="true"
			tabindex="-1"
		>
			{@render children()}
		</div>
	</div>
{:else}
	{@render children()}
{/if}
