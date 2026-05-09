<script lang="ts">
	import { cn } from '$lib/utils.js';

	type Size = 'sm' | 'md' | 'lg';

	interface Props {
		size?: Size;
		class?: string;
		label?: string;
	}

	let { size = 'md', class: className, label = 'Loading' }: Props = $props();

	const sizes: Record<Size, string> = {
		sm: 'h-3.5 w-3.5 border-2',
		md: 'h-4 w-4 border-2',
		lg: 'h-6 w-6 border-[3px]'
	};
</script>

<span
	role="status"
	aria-label={label}
	class={cn(
		'inline-block rounded-full border-muted-foreground/20 border-t-primary animate-spin shrink-0',
		sizes[size],
		className
	)}
></span>
