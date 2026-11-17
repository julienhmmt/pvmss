<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		variant?: 'color' | 'mono';
		showText?: boolean;
		size?: 'sm' | 'md' | 'lg' | 'xl';
		class?: string;
	}

	let { variant = 'color', showText = true, size = 'md', class: className = '' }: Props = $props();

	const sizeClass: Record<string, string> = {
		sm: 'h-4',
		md: 'h-6',
		lg: 'h-8',
		xl: 'h-16'
	};

	// 3x3 cluster grid; the accent cell (right edge, middle row) is the user's slice.
	const STEPS = [0, 36, 72];
	const cells = STEPS.flatMap((y) => STEPS.map((x) => ({ x, y, slice: x === 72 && y === 36 })));
</script>

<span class="inline-flex items-center gap-2 {className}" aria-label={m['chrome.sidebar.brand']()}>
	<svg
		role="img"
		aria-hidden={showText}
		aria-label={showText ? undefined : m['chrome.sidebar.brand']()}
		viewBox="0 0 100 100"
		class="{sizeClass[size]} w-auto"
		xmlns="http://www.w3.org/2000/svg"
	>
		{#each cells as cell (`${cell.x}-${cell.y}`)}
			<rect
				x={cell.x}
				y={cell.y}
				width="28"
				height="28"
				rx="6"
				class={cell.slice && variant === 'color' ? 'fill-primary' : 'fill-current'}
				opacity={!cell.slice && variant === 'mono' ? 0.45 : undefined}
			/>
		{/each}
	</svg>
	{#if showText}
		<span class="font-bold tracking-tight">{m['chrome.sidebar.brand']()}</span>
	{/if}
</span>
