<script lang="ts">
	/**
	 * CopyChip - a compact monospace chip that displays a technical value
	 * (MAC address, IP) and copies it on click, briefly showing "Copied!".
	 * For a labelled copy action next to a field, use CopyButton instead.
	 */
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		/** Value displayed on the chip and written to the clipboard. */
		value: string;
		/** data-testid passthrough. */
		testId?: string;
	}

	let { value, testId }: Props = $props();
	let copied = $state(false);

	async function copy(): Promise<void> {
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
</script>

<button
	type="button"
	class="rounded-md border border-border bg-muted/40 px-2 py-0.5 font-mono text-xs hover:bg-muted"
	onclick={() => void copy()}
	data-testid={testId}
>
	{copied ? m['common.copied']() : value}
</button>
