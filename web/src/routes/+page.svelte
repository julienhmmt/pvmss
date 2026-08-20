<script lang="ts">
	import HomeMarketing from '$lib/features/home/HomeMarketing.svelte';
	import HomeCta from '$lib/features/home/HomeCta.svelte';
	import HomeAdminQuickActions from '$lib/features/home/HomeAdminQuickActions.svelte';
	import HomeVmDashboard from '$lib/features/home/HomeVmDashboard.svelte';
	import HomeHowItWorks from '$lib/features/home/HomeHowItWorks.svelte';
	import BrandIcon from '$lib/shared/ui/icons/BrandIcon.svelte';
	import { m } from '$lib/paraglide/messages.js';
</script>

<section class="flex flex-col items-center gap-10 py-12">
	<div class="text-center">
		<div class="mb-4 inline-flex items-center justify-center rounded-2xl bg-primary/10 p-4 text-primary">
			<BrandIcon class="h-12 w-12" />
		</div>
		<h1 class="text-4xl font-semibold tracking-tight">{m['shell.title']()}</h1>
		<p class="mt-2 text-lg text-muted-foreground">{m['shell.subtitle']()}</p>
		<p class="mt-4 max-w-lg text-base text-muted-foreground">{m['home.hero.intro']()}</p>
	</div>
	<HomeCta />
	<HomeAdminQuickActions />
	<HomeHowItWorks />
	<HomeVmDashboard />
	<HomeMarketing />
</section>
