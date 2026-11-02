<script lang="ts">
	/**
	 * Tooltip - the shared info tooltip, built on bits-ui primitives
	 * (ui-review-remediation/03). bits-ui owns the floating positioning -
	 * flip and shift keep the bubble inside the viewport, which the previous
	 * hand-rolled placement could not do near the top edge - plus the
	 * Escape handling and the hover/focus triggers. Our shell keeps the
	 * 200ms delay, the warm bubble chrome and the same public props, so no
	 * call site changes. Feature code never imports bits-ui directly.
	 */
	import { Tooltip as BitsTooltip } from 'bits-ui';
	import type { Snippet } from 'svelte';

	interface Props {
		text: string;
		position?: 'top' | 'bottom';
		/** Non-interactive content (an info icon): the trigger wraps it. */
		children?: Snippet;
	}

	let { text, position = 'top', children }: Props = $props();
</script>

<BitsTooltip.Provider delayDuration={200}>
	<BitsTooltip.Root>
		<BitsTooltip.Trigger class="inline-flex cursor-default rounded-full pv-focus">
			{#if children}{@render children()}{/if}
		</BitsTooltip.Trigger>
		<BitsTooltip.Portal>
			<BitsTooltip.Content
				role="tooltip"
				side={position}
				sideOffset={8}
				collisionPadding={8}
				class="pointer-events-none z-[9999] w-max max-w-xs rounded-md border border-border bg-card px-2 py-1 text-xs text-foreground shadow-card"
			>
				{text}
			</BitsTooltip.Content>
		</BitsTooltip.Portal>
	</BitsTooltip.Root>
</BitsTooltip.Provider>
