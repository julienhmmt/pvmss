<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import { CAPABILITIES } from '$lib/features/capabilities/capability-data';
	import CapabilityCard from '$lib/features/capabilities/CapabilityCard.svelte';

	/**
	 * /about — public capabilities overview page. No auth required.
	 * Shows the full capabilities breakdown: 6 feature cards, the limits
	 * section with 3 sub-items, and an admin section note. Renders for
	 * everyone (anonymous, pool users, and admins).
	 */
	interface LimitItem {
		title: () => string;
		description: () => string;
	}

	const limits: LimitItem[] = [
		{ title: () => m['capabilities.limits.quota.title'](), description: () => m['capabilities.limits.quota.description']() },
		{ title: () => m['capabilities.limits.gabarit.title'](), description: () => m['capabilities.limits.gabarit.description']() },
		{ title: () => m['capabilities.limits.nodeCapacity.title'](), description: () => m['capabilities.limits.nodeCapacity.description']() }
	];
</script>

<svelte:head>
	<title>{m['capabilities.aboutTitle']()} — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-4xl px-4 py-8 md:px-6">
	<h1 class="text-2xl font-semibold tracking-tight">{m['capabilities.heading']()}</h1>
	<p class="mt-3 text-sm text-muted-foreground">{m['capabilities.page.introduction']()}</p>

	<div class="mt-8 grid gap-4 sm:grid-cols-2 lg:grid-cols-3" data-testid="about-capabilities-grid">
		{#each CAPABILITIES as capability (capability.id)}
			<CapabilityCard {capability} headingLevel="h2" />
		{/each}
	</div>

	<div class="mt-10" data-testid="about-limits-section">
		<h2 class="text-lg font-semibold tracking-tight">{m['capabilities.limits.heading']()}</h2>
		<div class="mt-4 grid gap-4 sm:grid-cols-3">
			{#each limits as limit (limit.title())}
				<div class="rounded-lg border border-border bg-card p-5 text-card-foreground">
					<h3 class="text-sm font-semibold tracking-tight">{limit.title()}</h3>
					<p class="mt-1.5 text-xs text-muted-foreground">{limit.description()}</p>
				</div>
			{/each}
		</div>
	</div>

	<div class="mt-10 rounded-lg border border-border bg-accent/20 p-5" data-testid="about-admin-section">
		<h2 class="text-lg font-semibold tracking-tight">{m['capabilities.page.adminSectionTitle']()}</h2>
		<p class="mt-2 text-sm text-muted-foreground">{m['capabilities.page.adminSectionDescription']()}</p>
	</div>

	<div class="mt-8">
		<a
			href={resolve('/')}
			class="rounded-md border border-border px-4 py-2 text-sm font-medium text-foreground transition-colors hover:bg-accent"
			data-testid="about-back-home"
		>
			{m['common.back']()}
		</a>
	</div>
</section>
