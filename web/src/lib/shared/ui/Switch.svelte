<script lang="ts">
	/**
	 * Switch — a small accessible toggle primitive in the same hand-rolled
	 * style as Dialog/Tabs/Dropdown (no bits-ui dependency in this codebase).
	 * Renders a role="switch" button with aria-checked, keyboard-activatable.
	 * The toggle's own transition is guarded by the global prefers-reduced-
	 * motion rule in app.css (constitution XII).
	 */
	interface Props {
		/** Accessible label for the switch. */
		label: string;
		/** Whether the switch is in the "on" position. */
		checked: boolean;
		/** Called when the user toggles the switch. */
		onToggle: () => void;
		/** Disables interaction (e.g. an unreadable template's enable direction). */
		disabled?: boolean;
	}

	let { label, checked, onToggle, disabled = false }: Props = $props();

	function handleClick(): void {
		onToggle();
	}

	function handleKeydown(event: KeyboardEvent): void {
		if (event.key === 'Enter' || event.key === ' ') {
			event.preventDefault();
			onToggle();
		}
	}
</script>

<button
	type="button"
	role="switch"
	aria-checked={checked}
	aria-label={label}
	{disabled}
	onclick={handleClick}
	onkeydown={handleKeydown}
	class="relative inline-flex h-5 w-9 shrink-0 cursor-pointer items-center rounded-full border border-border transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring disabled:cursor-not-allowed disabled:opacity-50"
	class:bg-primary={checked}
	class:bg-input={!checked}
>
	<span
		class="pointer-events-none block h-4 w-4 rounded-full bg-background shadow-sm transition-transform"
		class:translate-x-4={checked}
		class:translate-x-0.5={!checked}
	></span>
</button>
