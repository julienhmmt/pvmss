<script lang="ts">
	import { cn } from '$lib/utils.js';

	interface Props {
		message?: string | null;
		id?: string;
		class?: string;
	}

	let { message, id, class: className }: Props = $props();
</script>

{#if message}
	<p {id} role="alert" class={cn('text-xs text-destructive mt-1', className)}>{message}</p>
{/if}
