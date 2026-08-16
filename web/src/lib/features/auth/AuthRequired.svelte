<script lang="ts">
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import AuthBrandPanel from './AuthBrandPanel.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import Card from '$lib/shared/ui/Card.svelte';
</script>

<div class="grid w-full flex-1 grid-cols-1 lg:grid-cols-2">
	<AuthBrandPanel mode="warning" />

	<div class="flex flex-col items-center justify-center p-6 sm:p-10">
		<div class="w-full max-w-sm">
			<Card pad="lg" class="flex flex-col gap-6">
				<div class="flex flex-col items-center gap-4 text-center">
					<div
						class="rounded-full bg-warning p-3 text-warning-foreground"
						aria-hidden="true"
					>
						<svg
							viewBox="0 0 24 24"
							class="h-6 w-6"
							fill="none"
							stroke="currentColor"
							stroke-width="2"
							stroke-linecap="round"
							stroke-linejoin="round"
						>
							<path d="m21.73 18-8-14a2 2 0 0 0-3.48 0l-8 14A2 2 0 0 0 4 21h16a2 2 0 0 0 1.73-3Z" />
							<path d="M12 9v4" />
							<path d="M12 17h.01" />
						</svg>
					</div>
					<div>
						<h1 class="text-2xl font-semibold tracking-tight">{m['auth.requiredTitle']()}</h1>
						<p class="mt-2 text-sm text-muted-foreground">{m['auth.requiredDescription']()}</p>
					</div>
				</div>
				<Button onclick={() => void goto(resolve('/login'))}>
					{m['login.signIn']()}
				</Button>
			</Card>
		</div>
	</div>
</div>
