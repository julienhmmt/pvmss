<script lang="ts">
	import HomeMarketing from '$lib/features/home/HomeMarketing.svelte';
	import HomeCta from '$lib/features/home/HomeCta.svelte';
	import HomeVmDashboard from '$lib/features/home/HomeVmDashboard.svelte';
	import { m } from '$lib/paraglide/messages.js';
</script>

<section class="flex flex-col items-center gap-8 py-12">
	<div class="text-center">
		<h1 class="text-4xl font-semibold tracking-tight">{m['shell.title']()}</h1>
		<p class="mt-2 text-lg text-muted-foreground">{m['shell.subtitle']()}</p>
	</div>
	<HomeCta />
	<HomeVmDashboard />
	<HomeMarketing />
</section>
