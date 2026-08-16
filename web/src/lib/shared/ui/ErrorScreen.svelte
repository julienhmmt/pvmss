<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import BrandPanel from './BrandPanel.svelte';
	import Button from './Button.svelte';
	import Card from './Card.svelte';
	import { getErrorMessage } from '$lib/shared/error/error-messages';

	interface Props {
		status: number;
	}

	let { status }: Props = $props();

	const message = $derived(getErrorMessage(status));
</script>

{#snippet errorIcon(classes: string)}
	<svg viewBox="0 0 24 24" class={classes} fill="none" stroke="currentColor" stroke-width="2" stroke-linecap="round" stroke-linejoin="round" aria-hidden="true">
		<circle cx="12" cy="12" r="10" />
		<path d="m15 9-6 6" />
		<path d="m9 9 6 6" />
	</svg>
{/snippet}

<div class="grid w-full flex-1 grid-cols-1 lg:grid-cols-2">
	<BrandPanel mode="error" />

	<div class="flex flex-col items-center justify-center p-6 sm:p-10">
		<div class="w-full max-w-sm">
			<Card pad="lg" class="flex flex-col gap-6">
				<div class="flex flex-col items-center gap-4 text-center">
					<div
						class="rounded-full bg-destructive p-3 text-destructive-foreground"
						aria-hidden="true"
					>
						{@render errorIcon('h-6 w-6')}
					</div>
					<div>
						<h1 class="text-2xl font-semibold tracking-tight">{message.title()}</h1>
						<p class="mt-2 text-sm text-muted-foreground">{message.description()}</p>
					</div>
				</div>
				<Button onclick={() => void goto(resolve('/'))}>
					{m['error.backToHome']()}
				</Button>
			</Card>
		</div>
	</div>
</div>
