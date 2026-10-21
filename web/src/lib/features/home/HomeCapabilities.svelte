<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';
	import { CAPABILITIES, type CapabilityId } from '#lib/features/capabilities/capability-data.js';
	import CapabilityIcon from '#lib/features/capabilities/CapabilityIcon.svelte';
	import FeatureAdminIcon from '#lib/shared/ui/icons/FeatureAdminIcon.svelte';
	import FeatureMultiClusterIcon from '#lib/shared/ui/icons/FeatureMultiClusterIcon.svelte';
	import FeatureQuotasIcon from '#lib/shared/ui/icons/FeatureQuotasIcon.svelte';
	import FeatureSelfServiceIcon from '#lib/shared/ui/icons/FeatureSelfServiceIcon.svelte';
	import { reveal } from './reveal';

	/**
	 * HomeCapabilities - the product pitch as a server rack: each capability
	 * is a blade that slides into its slot as it scrolls in, and the sticky
	 * rack on wide screens lights the matching module. The three feature
	 * highlights lead, then the capability cards, then the admin preview.
	 * Multi-cluster is shown once (as a highlight); cloud-init is left out
	 * while it is paused (AGENTS.md, "Cloud-init").
	 */
	type Icon = 'selfService' | 'multiCluster' | 'quotas' | 'admin' | CapabilityId;

	interface Module {
		readonly icon: Icon;
		readonly title: () => string;
		readonly description: () => string;
	}

	const featureIcons = { selfService: FeatureSelfServiceIcon, multiCluster: FeatureMultiClusterIcon, quotas: FeatureQuotasIcon, admin: FeatureAdminIcon };

	const modules: readonly Module[] = [
		{ icon: 'selfService', title: () => m['home.feature.1.title'](), description: () => m['home.feature.1.description']() },
		{ icon: 'multiCluster', title: () => m['home.feature.2.title'](), description: () => m['home.feature.2.description']() },
		{ icon: 'quotas', title: () => m['home.feature.3.title'](), description: () => m['home.feature.3.description']() },
		...CAPABILITIES.filter((c) => c.id !== 'multiCluster' && c.id !== 'cloudinit').map((c) => ({
			icon: c.id,
			title: c.title,
			description: c.description
		})),
		{
			icon: 'admin',
			title: () => m['capabilities.page.adminSectionTitle'](),
			description: () => m['capabilities.page.adminSectionDescription']()
		}
	];

	let online = $state<readonly boolean[]>(modules.map(() => false));
	const onlineCount = $derived(online.filter(Boolean).length);

	function bringOnline(index: number): void {
		online = online.map((on, i) => on || i === index);
	}
</script>

{#snippet moduleIcon(icon: Icon)}
	{#if icon in featureIcons}
		{@const FeatureIcon = featureIcons[icon as keyof typeof featureIcons]}
		<FeatureIcon class="h-5 w-5" />
	{:else}
		<CapabilityIcon name={icon as CapabilityId} class="h-5 w-5" />
	{/if}
{/snippet}

<section class="mx-auto w-full max-w-6xl py-16" aria-labelledby="home-capabilities-heading">
	<div class="mx-auto max-w-2xl text-center" {@attach reveal()}>
		<p class="text-sm font-semibold tracking-wide text-primary">{m['home.features.ariaLabel']()}</p>
		<h2 id="home-capabilities-heading" class="mt-2 text-3xl font-semibold tracking-tight">
			{m['capabilities.heading']()}
		</h2>
		<p class="mt-3 text-muted-foreground">{m['capabilities.subtitle']()}</p>
	</div>

	<div class="mt-12 grid gap-10 lg:grid-cols-[17rem_1fr]">
		<!-- The rack: a sticky overview that fills up as the blades scroll in. -->
		<div class="hidden lg:block" aria-hidden="true">
			<div class="sticky top-24 rounded-xl border border-border bg-card p-3 shadow-[0_1px_2px_rgb(0_0_0/0.04),0_12px_32px_-16px_rgb(0_0_0/0.2)]">
				<div class="flex items-center justify-between px-1 pb-3 text-xs">
					<span class="font-mono font-medium text-foreground">pvmss-rack-01</span>
					<span class="inline-flex items-center gap-1.5 text-muted-foreground">
						<span class="h-1.5 w-1.5 rounded-full {onlineCount === modules.length ? 'bg-success' : 'pv-pulse bg-warning'}"></span>
						{m['home.rack.status']({ online: onlineCount, total: modules.length })}
					</span>
				</div>
				<ul class="flex flex-col gap-1.5 rounded-lg bg-muted p-1.5">
					{#each modules as module, i (i)}
						<li
							class="flex h-8 items-center gap-2 rounded-md border px-2 transition-all duration-700 ease-out {online[i]
								? 'translate-x-0 border-border bg-card opacity-100'
								: 'translate-x-6 border-transparent bg-transparent opacity-30'}"
						>
							<span class="h-1.5 w-1.5 shrink-0 rounded-full {online[i] ? 'bg-success' : 'bg-muted-foreground-subtle'}"></span>
							<span class="truncate text-xs font-medium text-foreground">{module.title()}</span>
							<span class="ml-auto flex gap-0.5">
								{#each [0, 1, 2, 3] as vent (vent)}
									<span class="h-3 w-0.5 rounded-full bg-border"></span>
								{/each}
							</span>
						</li>
					{/each}
				</ul>
			</div>
		</div>

		<ul class="grid gap-4 sm:grid-cols-2" data-testid="home-capabilities-grid">
			{#each modules as module, i (i)}
				<li
					class="pv-blade group relative flex overflow-hidden rounded-lg border border-border bg-card text-card-foreground"
					style:transition-delay="{(i % 2) * 120}ms"
					{@attach reveal(() => bringOnline(i))}
				>
					<!-- Rack ear: the screw holes that make a card read as a blade. -->
					<span class="flex w-6 shrink-0 flex-col items-center justify-between border-r border-border bg-muted py-3" aria-hidden="true">
						<span class="h-2 w-2 rounded-full border border-muted-foreground-subtle/50 bg-background"></span>
						<span class="font-mono text-[0.5625rem] text-muted-foreground-subtle [writing-mode:vertical-rl]">U{i + 1}</span>
						<span class="h-2 w-2 rounded-full border border-muted-foreground-subtle/50 bg-background"></span>
					</span>
					<div class="flex-1 p-6">
						<div class="flex items-start justify-between">
							<span class="inline-flex rounded-xl bg-primary/10 p-2.5 text-primary transition-transform duration-300 group-hover:scale-110">
								{@render moduleIcon(module.icon)}
							</span>
							<span
								class="mt-1 h-2 w-2 rounded-full transition-colors duration-500 {online[i] ? 'bg-success shadow-[0_0_0_4px] shadow-success/20' : 'bg-muted-foreground-subtle'}"
								aria-hidden="true"
							></span>
						</div>
						<h3 class="mt-4 text-base font-semibold tracking-tight">{module.title()}</h3>
						<p class="mt-1.5 text-sm text-muted-foreground">{module.description()}</p>
					</div>
				</li>
			{/each}
		</ul>
	</div>
</section>
