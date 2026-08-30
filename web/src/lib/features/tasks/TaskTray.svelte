<script lang="ts">
	import { getTaskTrayContext } from './tasks.svelte';
	import { m } from '$lib/paraglide/messages.js';

	// Navbar active-task tray (FR-015/FR-016): a counter badge while tasks are
	// in flight, plus a polite live region announcing the single toast fired
	// when a task reaches its terminal state (constitution XII).
	const tray = getTaskTrayContext();

	const toastClass = $derived(
		tray.toast === null
			? ''
			: tray.toast.kind === 'error'
				? 'border-destructive bg-background text-destructive'
				: tray.toast.kind === 'info'
					? 'border-info-soft-border bg-info-soft text-info-soft-foreground'
					: 'border-border bg-background text-foreground'
	);

	let dismissTimer: ReturnType<typeof setTimeout> | null = null;
	$effect(() => {
		if (tray.toast !== null) {
			if (dismissTimer !== null) clearTimeout(dismissTimer);
			dismissTimer = setTimeout(() => tray.clearToast(), 5000);
		}
	});
</script>

<div class="relative flex items-center">
	{#if tray.tasks.length > 0}
		<span
			role="status"
			aria-label={m['task.ariaLabel']({ count: tray.tasks.length })}
			class="inline-flex items-center gap-2 rounded-full border border-border bg-muted px-3 py-1 text-xs font-medium text-muted-foreground"
		>
			<span class="inline-block size-3 animate-spin rounded-full border-2 border-current border-t-transparent"></span>
			{tray.tasks.length}
		</span>
	{/if}

	<div aria-live="polite" class="absolute right-0 top-full z-50 mt-2">
		{#if tray.toast !== null}
			<div
				role="status"
				class="rounded-md border px-4 py-2 text-sm shadow-lg {toastClass}"
			>
				{tray.toast.message}
				<button
					type="button"
					class="ml-3 text-xs underline"
					onclick={() => tray.clearToast()}
				>
					{m['common.dismiss']()}
				</button>
			</div>
		{/if}
	</div>
</div>
