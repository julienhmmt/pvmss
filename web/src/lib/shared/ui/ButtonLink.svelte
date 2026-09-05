<script lang="ts">
	/**
	 * ButtonLink — the anchor-shaped twin of Button.
	 * Use for navigation CTAs that need the button look but must keep
	 * link semantics (screen-reader role, right-click, href).
	 *
	 * Variants, sizes and states are copied from Button deliberately: the
	 * two must stay indistinguishable, because which one a call site needs
	 * is a semantics decision, never a visual one.
	 */
	import type { Snippet } from 'svelte';
	import { focusOnMount } from './focus-on-mount';

	type Variant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'subtle' | 'destructive';
	type Size = 'sm' | 'md' | 'lg' | 'icon' | 'icon-sm';

	interface Props {
		href: string;
		variant?: Variant;
		size?: Size;
		download?: string | boolean;
		target?: string;
		rel?: string;
		/** Stretch to the container width. */
		block?: boolean;
		focusOnMount?: boolean;
		/** Accessible label, when the visible content is not descriptive enough. */
		label?: string;
		/** Icon rendered before the label. */
		icon?: Snippet;
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
		block = false,
		focusOnMount: shouldFocusOnMount = false,
		label,
		icon,
		class: extra = '',
		children,
		...rest
	}: Props = $props();

	const base =
		'inline-flex shrink-0 items-center justify-center gap-2 rounded-[var(--radius-control)] font-semibold ' +
		'transition-[background-color,border-color,color,transform] duration-150 ' +
		'pv-focus ' +
		'active:translate-y-px';

	const sizes: Record<Size, string> = {
		sm: 'h-8 px-3 text-xs',
		md: 'h-10 px-4 text-sm',
		lg: 'h-11 px-5 text-sm',
		icon: 'h-10 w-10 text-sm',
		'icon-sm': 'h-8 w-8 text-xs'
	};

	const variants: Record<Variant, string> = {
		primary: 'bg-primary text-primary-foreground hover:bg-primary/90 active:bg-primary/95',
		secondary: 'border border-border bg-card text-foreground hover:border-muted-foreground-subtle hover:bg-muted',
		outline: 'border border-border bg-transparent text-foreground hover:border-muted-foreground-subtle hover:bg-muted/60',
		ghost: 'text-muted-foreground hover:bg-accent hover:text-accent-foreground',
		subtle: 'bg-muted text-foreground hover:bg-border',
		destructive: 'bg-destructive text-destructive-foreground hover:bg-destructive/90'
	};
</script>

<a
	{href}
	{download}
	{target}
	{rel}
	aria-label={label}
	class="{base} {sizes[size]} {variants[variant]} {block ? 'w-full' : ''} {extra}"
	use:focusOnMount={shouldFocusOnMount}
	{...rest}
>
	{#if icon}{@render icon()}{/if}
	{@render children()}
</a>
