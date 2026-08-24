<script lang="ts">
	/**
	 * ButtonLink — the anchor-shaped twin of Button.
	 * Use for navigation CTAs that need the button look but must keep
	 * link semantics (screen-reader role, right-click, href).
	 */
	import type { Snippet } from 'svelte';
	import { focusOnMount } from './focus-on-mount';

	type Variant = 'primary' | 'secondary' | 'ghost' | 'destructive';
	type Size = 'sm' | 'md';

	interface Props {
		href: string;
		variant?: Variant;
		size?: Size;
		download?: string | boolean;
		target?: string;
		rel?: string;
		focusOnMount?: boolean;
		/** Accessible label, when the visible content is not descriptive enough. */
		label?: string;
		class?: string;
		children: Snippet;
		[key: string]: unknown;
	}

	let {
		href,
		variant = 'primary',
		size = 'md',
		download = undefined,
		target = undefined,
		rel = undefined,
		focusOnMount: shouldFocusOnMount = false,
		label,
		class: extra = '',
		children,
		...rest
	}: Props = $props();

	const base =
		'inline-flex items-center justify-center rounded-[0.625rem] font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background';

	const sizes: Record<Size, string> = {
		sm: 'px-3 py-1.5 text-xs',
		md: 'px-4 py-2.5 text-sm'
	};

	const variants: Record<Variant, string> = {
		primary: 'bg-primary text-primary-foreground shadow-card hover:bg-primary/90',
		secondary: 'border border-border bg-card text-foreground hover:bg-muted',
		ghost: 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
		destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90'
	};
</script>

<a
	{href}
	{download}
	{target}
	{rel}
	aria-label={label}
	class="{base} {sizes[size]} {variants[variant]} {extra}"
	use:focusOnMount={shouldFocusOnMount}
	{...rest}
>
	{@render children()}
</a>
