<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import HomeCapabilities from '$lib/features/home/HomeCapabilities.svelte';
	import HomeCta from '$lib/features/home/HomeCta.svelte';
	import HomeVmDashboard from '$lib/features/home/HomeVmDashboard.svelte';
	import HomeHowItWorks from '$lib/features/home/HomeHowItWorks.svelte';
	import Logo from '$lib/shared/ui/Logo.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import { m } from '$lib/paraglide/messages.js';

	const session = getSessionContext();

	onMount(() => {
		if (session.principal?.isAdmin) {
			void goto(resolve('/admin'));
		}
	});
</script>

{#if !session.principal?.isAdmin}
	<section class="flex flex-col items-center gap-10 py-12">
		{#if !session.principal}
			<!-- The product pitch: name, tagline, "how it works", capabilities.
			     Anonymous visitors only — a signed-in user already uses PVMSS
			     and lands here to check their VMs, not to be resold the product. -->
			<div class="text-center">
				<div class="mb-4 inline-flex items-center justify-center rounded-2xl bg-primary/10 p-4 text-primary">
					<Logo variant="color" showText={false} size="xl" />
				</div>
				<h1 class="text-4xl font-semibold tracking-tight">{m['shell.title']()}</h1>
				<p class="mt-2 text-lg text-muted-foreground">{m['shell.subtitle']()}</p>
			</div>
		{/if}
		<HomeCta />
		<HomeHowItWorks />
		<HomeVmDashboard />
		<HomeCapabilities />
	</section>
{/if}
