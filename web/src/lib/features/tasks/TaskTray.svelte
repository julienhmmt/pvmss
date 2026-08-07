<script lang="ts">
	import { getTaskTrayContext } from './tasks.svelte';

	// Navbar active-task tray (FR-015/FR-016): a counter badge while tasks are
	// in flight, plus a polite live region announcing the single toast fired
	// when a task reaches its terminal state (constitution XII).
	const tray = getTaskTrayContext();

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
			aria-label="{tray.tasks.length} task(s) in progress"
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
				class="rounded-md border px-4 py-2 text-sm shadow-lg {tray.toast.kind === 'success'
					? 'border-border bg-background text-foreground'
					: 'border-destructive bg-background text-destructive'}"
			>
				{tray.toast.message}
				<button
					type="button"
					class="ml-3 text-xs underline"
					onclick={() => tray.clearToast()}
				>
					Dismiss
				</button>
			</div>
		{/if}
	</div>
</div>
