<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setConsoleContext } from '$lib/features/vm-console/console.svelte';
	import VmConsole from '$lib/features/vm-console/VmConsole.svelte';
	import ConsoleToolbar from '$lib/features/vm-console/ConsoleToolbar.svelte';

	const cluster = page.params.cluster ?? 'default';
	const vmid = Number(page.params.vmid);

	const store = setConsoleContext(cluster, vmid);

	onMount(() => {
		// The connect happens inside VmConsole.svelte's onMount, which runs
		// after the container element is bound.
	});

	onDestroy(() => {
		store.disconnect();
	});
</script>

<svelte:head>
	<title>VM {vmid} Console — PVMSS</title>
</svelte:head>

<section class="mx-auto flex h-screen w-full max-w-6xl flex-col px-4 py-4">
	<div class="mb-3 flex items-center gap-3">
		<a
			href={resolve('/vms/[cluster]/[vmid]', { cluster, vmid: String(vmid) })}
			class="inline-flex items-center text-sm text-muted-foreground hover:text-foreground"
		>
			← Back to VM
		</a>
		<h1 class="text-lg font-semibold" data-testid="vm-console-title">
			VM {vmid} Console
		</h1>
		<span
			class="inline-flex items-center rounded-full px-2 py-0.5 text-xs {
				store.state === 'connected'
					? 'bg-success-soft text-success-soft-foreground'
					: store.state === 'error'
						? 'bg-destructive-soft text-destructive-soft-foreground'
						: 'bg-muted text-muted-foreground'
			}"
			aria-live="polite"
			data-testid="vm-console-status"
		>
			{store.state}
		</span>
	</div>

	<ConsoleToolbar />

	<div class="mt-3 flex-1 overflow-hidden">
		<svelte:boundary>
			<VmConsole />
			{#snippet failed(error)}
				<div
					class="flex h-full w-full flex-col items-center justify-center gap-2 rounded-md border border-border bg-destructive-soft p-4 text-sm text-destructive"
					data-testid="vm-console-boundary-fallback"
				>
					<p>The console crashed unexpectedly.</p>
					<p class="text-xs text-muted-foreground">{error instanceof Error ? error.message : String(error)}</p>
					<button
						type="button"
						class="rounded-md border border-border bg-background px-3 py-1.5 text-sm text-foreground hover:bg-muted"
						onclick={() => window.location.reload()}
					>
						Reload page
					</button>
				</div>
			{/snippet}
		</svelte:boundary>
	</div>
</section>
