<script lang="ts">
	import type { Snippet } from 'svelte';
	import { m } from '$lib/paraglide/messages.js';
	import Button from './Button.svelte';
	import ErrorIcon from './icons/ErrorIcon.svelte';

	/**
	 * ErrorState — a consistent error display for store-level failures.
	 * Shows an error icon, a title, an optional description, and an optional
	 * retry action. Communicates the error as an alert to assistive technologies.
	 */
	interface Props {
		title: string;
		description?: string;
		icon?: Snippet;
		retry?: () => void;
		retryLabel?: string;
	}

	let {
		title,
		description,
		icon,
		retry,
		retryLabel = m['common.retry']()
	}: Props = $props();
</script>

<div
	role="alert"
	aria-live="assertive"
	class="flex flex-col items-center justify-center gap-3 px-4 py-12 text-center"
>
	{#if icon}
		<div class="text-destructive">{@render icon()}</div>
	{:else}
		<div class="text-destructive" aria-hidden="true">
			<ErrorIcon class="h-10 w-10" />
		</div>
	{/if}
	<div>
		<p class="font-semibold">{title}</p>
		{#if description}
			<p class="mt-1 text-sm text-muted-foreground">{description}</p>
		{/if}
	</div>
	{#if retry}
		<div class="mt-2">
			<Button onclick={retry}>{retryLabel}</Button>
		</div>
	{/if}
</div>
