<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import BrandPanel from '$lib/shared/ui/BrandPanel.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
	import WarningIcon from '$lib/shared/ui/icons/WarningIcon.svelte';
</script>

<div class="grid w-full flex-1 grid-cols-1 lg:grid-cols-2">
	<BrandPanel mode="warning" />

	<div class="flex flex-col items-center justify-center p-6 sm:p-10">
		<div class="w-full max-w-sm">
			<section
				role="status"
				aria-live="polite"
				aria-label={m['auth.requiredTitle']()}
			>
				<Card pad="lg" class="flex flex-col gap-6">
					<div class="flex flex-col items-center gap-4 text-center">
						<div
							class="rounded-full bg-warning p-3 text-warning-foreground"
							aria-hidden="true"
						>
							<WarningIcon class="h-6 w-6" />
						</div>
						<div>
							<h1 class="text-2xl font-semibold tracking-tight">{m['auth.requiredTitle']()}</h1>
							<p class="mt-2 text-sm text-muted-foreground">{m['auth.requiredDescription']()}</p>
						</div>
					</div>
					<Button focusOnMount onclick={() => void goto(resolve('/login'))}>
						{m['login.signIn']()}
					</Button>
				</Card>
			</section>
		</div>
	</div>
</div>
