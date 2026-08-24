<script lang="ts">
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import { CAPABILITIES } from '$lib/features/capabilities/capability-data';
	import CapabilityIcon from '$lib/features/capabilities/CapabilityIcon.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';
	import FeatureAdminIcon from '$lib/shared/ui/icons/FeatureAdminIcon.svelte';
	import FeatureSelfServiceIcon from '$lib/shared/ui/icons/FeatureSelfServiceIcon.svelte';
	import StepNumberIcon from '$lib/shared/ui/icons/StepNumberIcon.svelte';
	import ButtonLink from '$lib/shared/ui/ButtonLink.svelte';

	/**
	 * /about — public product overview page. No auth required.
	 * Long-form article: what PVMSS is, what it does, how it works,
	 * who it is for, limits, and a call to action. Avoids duplicating
	 * the home page's 3x3 card grid.
	 */
	const session = getSessionContext();

	interface LimitItem {
		title: () => string;
		description: () => string;
	}

	interface Step {
		number: number;
		title: () => string;
		description: () => string;
	}

	const limits: LimitItem[] = [
		{
			title: () => m['capabilities.limits.quota.title'](),
			description: () => m['capabilities.limits.quota.description']()
		},
		{
			title: () => m['capabilities.limits.gabarit.title'](),
			description: () => m['capabilities.limits.gabarit.description']()
		},
		{
			title: () => m['capabilities.limits.nodeCapacity.title'](),
			description: () => m['capabilities.limits.nodeCapacity.description']()
		}
	];

	const steps: Step[] = [
		{ number: 1, title: () => m['home.steps.1.title'](), description: () => m['home.steps.1.description']() },
		{ number: 2, title: () => m['home.steps.2.title'](), description: () => m['home.steps.2.description']() },
		{ number: 3, title: () => m['home.steps.3.title'](), description: () => m['home.steps.3.description']() }
	];
</script>

<svelte:head>
	<title>{m['capabilities.aboutTitle']()} — PVMSS</title>
</svelte:head>

<article class="mx-auto w-full max-w-3xl px-4 py-8 md:px-6" data-testid="about-page">
	<header>
		<h1 class="text-2xl font-semibold tracking-tight">{m['capabilities.aboutTitle']()}</h1>
		<p class="mt-3 max-w-prose text-base leading-relaxed text-muted-foreground">
			{m['capabilities.page.introduction']()}
		</p>
	</header>

	<section class="mt-10" aria-labelledby="about-capabilities-heading">
		<h2 id="about-capabilities-heading" class="text-lg font-semibold tracking-tight">
			{m['capabilities.heading']()}
		</h2>
		<p class="mt-2 max-w-prose text-sm leading-relaxed text-muted-foreground">
			{m['capabilities.subtitle']()}
		</p>

		<ul class="mt-5 space-y-4" role="list" data-testid="about-capabilities-list">
			{#each CAPABILITIES as capability (capability.id)}
				<li class="flex items-start gap-4">
					<div
						class="inline-flex h-9 w-9 shrink-0 items-center justify-center rounded-xl bg-primary/10 text-primary"
					>
						<CapabilityIcon name={capability.id} class="h-5 w-5" />
					</div>
					<div>
						<h3 class="text-sm font-semibold text-foreground">{capability.title()}</h3>
						<p class="mt-0.5 max-w-prose text-sm leading-relaxed text-muted-foreground">
							{capability.description()}
						</p>
					</div>
				</li>
			{/each}
		</ul>
	</section>

	<section class="mt-10" aria-labelledby="about-steps-heading">
		<h2 id="about-steps-heading" class="text-lg font-semibold tracking-tight">
			{m['home.steps.heading']()}
		</h2>

		<ol class="mt-5 space-y-4" role="list" data-testid="about-steps-list">
			{#each steps as step (step.number)}
				<li class="flex items-start gap-4">
					<StepNumberIcon number={step.number} class="h-9 w-9 shrink-0" />
					<div>
						<h3 class="text-sm font-semibold text-foreground">{step.title()}</h3>
						<p class="mt-0.5 max-w-prose text-sm leading-relaxed text-muted-foreground">
							{step.description()}
						</p>
					</div>
				</li>
			{/each}
		</ol>
	</section>

	<section class="mt-10" aria-labelledby="about-audience-heading">
		<h2 id="about-audience-heading" class="text-lg font-semibold tracking-tight">
			{m['about.audience.heading']()}
		</h2>
		<div class="mt-4 grid gap-6 sm:grid-cols-2" data-testid="about-audience-grid">
			<div class="flex flex-col gap-3">
				<div class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary">
					<FeatureSelfServiceIcon class="h-5 w-5" />
				</div>
				<h3 class="text-sm font-semibold text-foreground">{m['about.audience.userTitle']()}</h3>
				<p class="max-w-prose text-sm leading-relaxed text-muted-foreground">
					{m['about.audience.userDescription']()}
				</p>
			</div>
			<div class="flex flex-col gap-3">
				<div class="inline-flex h-9 w-9 items-center justify-center rounded-xl bg-primary/10 text-primary">
					<FeatureAdminIcon class="h-5 w-5" />
				</div>
				<h3 class="text-sm font-semibold text-foreground">{m['capabilities.page.adminSectionTitle']()}</h3>
				<p class="max-w-prose text-sm leading-relaxed text-muted-foreground">
					{m['capabilities.page.adminSectionDescription']()}
				</p>
			</div>
		</div>
	</section>

	<section class="mt-10" aria-labelledby="about-limits-heading">
		<h2 id="about-limits-heading" class="text-lg font-semibold tracking-tight">
			{m['capabilities.limits.heading']()}
		</h2>
		<dl class="mt-4 space-y-3" data-testid="about-limits-list">
			{#each limits as limit (limit.title())}
				<div>
					<dt class="text-sm font-semibold text-foreground">{limit.title()}</dt>
					<dd class="max-w-prose text-sm leading-relaxed text-muted-foreground">{limit.description()}</dd>
				</div>
			{/each}
		</dl>
	</section>

	<section class="mt-10" aria-labelledby="about-cta-heading">
		<h2 id="about-cta-heading" class="text-lg font-semibold tracking-tight">{m['about.cta.heading']()}</h2>
		<p class="mt-2 max-w-prose text-sm leading-relaxed text-muted-foreground">{m['about.cta.description']()}</p>

		<div class="mt-4 flex flex-wrap gap-3" data-testid="about-cta-actions">
			{#if !session.principal}
				<ButtonLink href={resolve('/login')}>
					{m['home.cta.login']()}
				</ButtonLink>
				<ButtonLink href={resolve('/docs')} variant="secondary">
					{m['home.cta.documentation']()}
				</ButtonLink>
			{:else if session.principal.isAdmin}
				<ButtonLink href={resolve('/admin')}>
					{m['chrome.sidebar.navAdmin']()}
				</ButtonLink>
				<ButtonLink href={resolve('/docs')} variant="secondary">
					{m['home.cta.documentation']()}
				</ButtonLink>
			{:else}
				<ButtonLink href={resolve('/vms')}>
					{m['home.cta.my_vms']()}
				</ButtonLink>
				<ButtonLink href={resolve('/vms/create')} variant="secondary">
					{m['home.cta.create_vm']()}
				</ButtonLink>
				<ButtonLink href={resolve('/docs')} variant="secondary">
					{m['home.cta.documentation']()}
				</ButtonLink>
			{/if}
		</div>
	</section>

	<div class="mt-10">
		<ButtonLink
			href={resolve('/')}
			variant="secondary"
			data-testid="about-back-home"
		>
			{m['common.back']()}
		</ButtonLink>
	</div>
</article>
