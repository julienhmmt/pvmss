<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import BrandPanel from './BrandPanel.svelte';
	import Button from './Button.svelte';
	import Card from './Card.svelte';
	import ErrorIcon from './icons/ErrorIcon.svelte';
	import { getErrorMessage } from '$lib/shared/error/error-messages';

	interface Props {
		status: number;
		message?: string | undefined;
	}

	let { status, message }: Props = $props();

	const errorMessage = $derived(getErrorMessage(status));
</script>

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
						<ErrorIcon class="h-6 w-6" />
					</div>
					<div>
						<h1 class="text-2xl font-semibold tracking-tight">{errorMessage.title()}</h1>
						<p class="mt-2 text-sm text-muted-foreground">
							{message ?? errorMessage.description()}
						</p>
					</div>
				</div>
				<Button onclick={() => void goto(resolve('/'))}>
					{m['error.backToHome']()}
				</Button>
			</Card>
		</div>
	</div>
</div>
