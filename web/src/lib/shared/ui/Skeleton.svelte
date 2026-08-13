<script lang="ts">
	/**
	 * Skeleton — a single pulsing placeholder block (`bg-muted animate-pulse
	 * rounded-md`), sized via the `class` prop. Building block, not
	 * table-specific. The pulse is zeroed by the global prefers-reduced-motion
	 * rule in app.css, so no extra guard is needed here.
	 */
	interface Props {
		class?: string;
	}

	let { class: className = 'h-4 w-full' }: Props = $props();
</script>

<div class="bg-muted animate-pulse rounded-md {className}" aria-hidden="true"></div>
