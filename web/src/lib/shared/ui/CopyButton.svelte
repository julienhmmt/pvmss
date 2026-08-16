<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';
	import Button from './Button.svelte';

	interface Props {
		value: string;
	}

	let { value }: Props = $props();
	let copied = $state(false);

	async function handleCopy(): Promise<void> {
		await navigator.clipboard.writeText(value);
		copied = true;
		setTimeout(() => {
			copied = false;
		}, 1500);
	}
</script>

<Button size="sm" onclick={() => void handleCopy()}>
	{copied ? m['common.copied']() : m['common.copy']()}
</Button>
