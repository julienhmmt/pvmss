<script lang="ts">
	/**
	 * Menu - the shared overflow-menu primitive, built on bits-ui's
	 * DropdownMenu (ui-review-remediation/04): floating positioning with
	 * collision handling, arrow-key navigation, Escape, focus return and
	 * `role="menu"` semantics come from the library; the kit owns the chrome
	 * (card surface, control radius, `.pv-focus`, elevation ramp) and the
	 * item shape. Feature code never imports bits-ui directly.
	 *
	 * Items carry an optional icon and a `destructive` flag for actions like
	 * Delete - the flag tints the label, it never skips the caller's own
	 * confirmation step.
	 */
	import { DropdownMenu } from 'bits-ui';
	import type { Component, Snippet } from 'svelte';
	import MoreIcon from './icons/MoreIcon.svelte';

	export interface MenuItem {
		label: string;
		onclick: () => void;
		/** Tinted destructive entry (delete-style actions). */
		destructive?: boolean;
		disabled?: boolean;
		icon?: Component<{ class?: string }>;
		testId?: string;
	}

	interface Props {
		/** Accessible label on the ellipsis trigger. */
		label: string;
		items: readonly MenuItem[];
		testId?: string;
		/** Custom trigger content; defaults to the ellipsis icon. */
		trigger?: Snippet;
	}

	let { label, items, testId, trigger }: Props = $props();
</script>

<DropdownMenu.Root>
	<DropdownMenu.Trigger
		aria-label={label}
		data-testid={testId}
		class="inline-flex h-8 w-8 shrink-0 items-center justify-center rounded-[var(--radius-control)] border border-muted-foreground-subtle/50 bg-card text-foreground transition-colors pv-focus hover:border-muted-foreground-subtle hover:bg-muted"
	>
		{#if trigger}
			{@render trigger()}
		{:else}
			<MoreIcon class="h-4 w-4" />
		{/if}
	</DropdownMenu.Trigger>
	<DropdownMenu.Portal>
		<DropdownMenu.Content
			sideOffset={4}
			collisionPadding={8}
			align="end"
			class="z-[9999] min-w-40 rounded-lg border border-border bg-card p-1 text-card-foreground shadow-overlay"
		>
			{#each items as item (item.label)}
				<DropdownMenu.Item
					disabled={item.disabled ?? false}
					onSelect={() => item.onclick()}
					data-testid={item.testId}
					class="flex cursor-pointer items-center gap-2 rounded-[var(--radius-control)] px-2 py-1.5 text-sm outline-none select-none data-[disabled]:pointer-events-none data-[disabled]:opacity-50 data-[highlighted]:bg-muted {item.destructive
						? 'text-destructive'
						: 'text-foreground'}"
				>
					{#if item.icon}
						<item.icon class="h-4 w-4" />
					{/if}
					{item.label}
				</DropdownMenu.Item>
			{/each}
		</DropdownMenu.Content>
	</DropdownMenu.Portal>
</DropdownMenu.Root>
