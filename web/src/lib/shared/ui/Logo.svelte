<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';

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
</script>

<span class="inline-flex items-center gap-2 {className}" aria-label={m['chrome.sidebar.brand']()}>
	<svg
		role="img"
		aria-hidden={showText}
		aria-label={showText ? undefined : m['chrome.sidebar.brand']()}
		viewBox="0 0 64 96"
		class="{sizeClass[size]} w-auto"
		xmlns="http://www.w3.org/2000/svg"
	>
		<defs>
			{#if variant === 'color'}
				<linearGradient id="logo-gradient" x1="0" y1="0" x2="1" y2="1">
					<stop offset="0" stop-color="#FF8A33" />
					<stop offset="1" stop-color="#C25E00" />
				</linearGradient>
			{/if}
		</defs>
		<g transform="translate(0,16)">
			<rect
				x="4"
				y="44"
				width="56"
				height="12"
				rx="4"
				fill={variant === 'color' ? 'url(#logo-gradient)' : 'currentColor'}
				opacity="0.45"
			/>
			<rect
				x="4"
				y="28"
				width="56"
				height="12"
				rx="4"
				fill={variant === 'color' ? 'url(#logo-gradient)' : 'currentColor'}
				opacity="0.72"
			/>
			<rect
				x="4"
				y="12"
				width="56"
				height="12"
				rx="4"
				fill={variant === 'color' ? 'url(#logo-gradient)' : 'currentColor'}
			/>
		</g>
	</svg>
	{#if showText}
		<span class="font-bold tracking-tight">{m['chrome.sidebar.brand']()}</span>
	{/if}
</span>
