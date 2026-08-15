<script lang="ts">
	/**
	 * Button — the shared button primitive, in the same hand-rolled style as
	 * Switch/Dialog/Tabs (no bits-ui in this codebase). Gives every admin
	 * button one vocabulary and a real focus-visible ring (product register:
	 * every interactive control needs default/hover/focus/disabled states).
	 */
	import type { Snippet } from 'svelte';

	type Variant = 'primary' | 'secondary' | 'ghost' | 'destructive';
	type Size = 'sm' | 'md';

	interface Props {
		variant?: Variant;
		size?: Size;
		type?: 'button' | 'submit';
		disabled?: boolean;
		/** Accessible label, when the visible content is not descriptive enough. */
		label?: string;
		onclick?: () => void;
		children: Snippet;
	}

	let {
		variant = 'primary',
		size = 'md',
		type = 'button',
		disabled = false,
		label,
		onclick,
		children
	}: Props = $props();

	const base =
		'inline-flex items-center justify-center rounded-[0.625rem] font-semibold transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background disabled:pointer-events-none disabled:opacity-50';

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

<button
	{type}
	{disabled}
	aria-label={label}
	{onclick}
	class="{base} {sizes[size]} {variants[variant]}"
>
	{@render children()}
</button>
