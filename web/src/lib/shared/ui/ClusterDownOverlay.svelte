<script lang="ts">
	import type { Snippet } from 'svelte';
	import { getStatusContext } from '$lib/features/chrome/status.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import { post } from '$lib/shared/api/client';
	import Button from './Button.svelte';

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
				<div class="mt-4">
					<Button onclick={() => void retry()} data-testid="cluster-down-retry">
						{m['cluster.down.retry']()}
					</Button>
				</div>
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
