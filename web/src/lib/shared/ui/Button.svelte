<script lang="ts">
	/**
	 * Button — the shared button primitive, in the same hand-rolled style as
	 * Switch/Dialog/Tabs (no bits-ui in this codebase). Gives every admin
	 * button one vocabulary and a real focus-visible ring (product register:
	 * every interactive control needs default/hover/focus/disabled states).
	 *
	 * Variants map to intent, not to colour: `primary` is the one action a
	 * screen is about, `secondary` is the bordered default, `outline` is the
	 * quieter bordered form on tinted grounds, `ghost` is toolbar-weight,
	 * `subtle` is a filled neutral for dense rows, `destructive` is delete.
	 * Per DESIGN.md's flat-by-default rule none of them carry a shadow at
	 * rest; depth comes from a 1px press translate instead.
	 */
	import type { Snippet } from 'svelte';
	import { focusOnMount } from './focus-on-mount';
	import SpinnerIcon from './icons/SpinnerIcon.svelte';

	type Variant = 'primary' | 'secondary' | 'outline' | 'ghost' | 'subtle' | 'destructive';
	type Size = 'sm' | 'md' | 'lg' | 'icon' | 'icon-sm';

	interface Props {
		variant?: Variant;
		size?: Size;
		type?: 'button' | 'submit';
		disabled?: boolean;
		loading?: boolean;
		/** Stretch to the container width (dialog footers, mobile forms). */
		block?: boolean;
		focusOnMount?: boolean;
		/** Accessible label, when the visible content is not descriptive enough. */
		label?: string;
		/** Icon rendered before the label. Swapped for the spinner while loading. */
		icon?: Snippet;
		onclick?: () => void;
		class?: string;
		children: Snippet;
		[key: string]: unknown;
	}

	let {
		variant = 'primary',
		size = 'md',
		type = 'button',
		disabled = false,
		loading = false,
		block = false,
		focusOnMount: shouldFocusOnMount = false,
		label,
		icon,
		onclick,
		class: klass = '',
		children,
		...rest
	}: Props = $props();

	const base =
		'inline-flex shrink-0 items-center justify-center gap-2 rounded-[var(--radius-control)] font-semibold ' +
		'transition-[background-color,border-color,color,transform] duration-150 ' +
		'pv-focus ' +
		'active:translate-y-px disabled:pointer-events-none disabled:opacity-50 disabled:active:translate-y-0';

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

	const isIconOnly = $derived(size === 'icon' || size === 'icon-sm');
</script>

<button
	{type}
	disabled={disabled || loading}
	aria-busy={loading ? 'true' : undefined}
	aria-label={label}
	{onclick}
	class="{base} {sizes[size]} {variants[variant]} {block ? 'w-full' : ''} {klass}"
	use:focusOnMount={shouldFocusOnMount}
	{...rest}
>
	{#if loading}
		<SpinnerIcon class="h-4 w-4" />
	{:else if icon}
		{@render icon()}
	{/if}
	{#if !(isIconOnly && loading)}
		{@render children()}
	{/if}
</button>
