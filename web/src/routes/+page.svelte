<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import HomeMarketing from '$lib/features/home/HomeMarketing.svelte';
	import HomeCta from '$lib/features/home/HomeCta.svelte';
	import HomeVmDashboard from '$lib/features/home/HomeVmDashboard.svelte';
	import HomeHowItWorks from '$lib/features/home/HomeHowItWorks.svelte';
	import BrandIcon from '$lib/shared/ui/icons/BrandIcon.svelte';
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
		<div class="text-center">
			<div class="mb-4 inline-flex items-center justify-center rounded-2xl bg-primary/10 p-4 text-primary">
				<BrandIcon class="h-12 w-12" />
			</div>
			<h1 class="text-4xl font-semibold tracking-tight">{m['shell.title']()}</h1>
			<p class="mt-2 text-lg text-muted-foreground">{m['shell.subtitle']()}</p>
		</div>
		<HomeCta />
		<HomeHowItWorks />
		<HomeVmDashboard />
		<HomeMarketing />
	</section>
{/if}
