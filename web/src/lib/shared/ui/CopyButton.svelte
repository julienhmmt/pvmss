<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';
	import Button from './Button.svelte';

	interface Props {
		value: string;
		/** Button variant - defaults to Button's primary. */
		variant?: 'primary' | 'secondary' | 'outline' | 'ghost' | 'subtle' | 'destructive' | 'warning' | 'link';
		/** data-testid passthrough. */
		testId?: string;
	}

	let { value, variant = 'primary', testId }: Props = $props();
	let copied = $state(false);

	async function handleCopy(): Promise<void> {
		try {
			await navigator.clipboard.writeText(value);
			copied = true;
			setTimeout(() => {
				copied = false;
			}, 1500);
		} catch {
			copied = false;
		}
	}

	const label = $derived(copied ? m['common.copied']() : m['common.copy']());
</script>

<Button size="sm" {variant} onclick={() => void handleCopy()} data-testid={testId}>
	{label}
</Button>
