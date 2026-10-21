<script lang="ts">
	import { m } from '#lib/paraglide/messages.js';

	/**
	 * HomeHeroMock - a decorative miniature of the machine list: a few
	 * machines humming along and one booting, on a loop. Purely illustrative
	 * (aria-hidden); the names are sample data, not anyone's inventory.
	 */
	const rows = [
		{ name: 'web-frontend-01', node: 'pve-01', state: 'running', load: 'w-3/5', speed: '4.2s' },
		{ name: 'ci-runner-02', node: 'pve-02', state: 'booting', load: 'w-2/5', speed: '3.1s' },
		{ name: 'postgres-main', node: 'pve-01', state: 'running', load: 'w-4/5', speed: '5.6s' },
		{ name: 'sandbox-lab', node: 'pve-03', state: 'stopped', load: 'w-0', speed: '0s' }
	] as const;
</script>

<div class="relative" aria-hidden="true">
	<div class="pv-dot-grid absolute -inset-6 rounded-3xl opacity-70"></div>
	<div class="relative overflow-hidden rounded-xl border border-border bg-card shadow-[0_1px_2px_rgb(0_0_0/0.04),0_12px_40px_-12px_rgb(0_0_0/0.18)]">
		<div class="flex items-center gap-2 border-b border-border px-4 py-3">
			<span class="h-2.5 w-2.5 rounded-full bg-border"></span>
			<span class="h-2.5 w-2.5 rounded-full bg-border"></span>
			<span class="h-2.5 w-2.5 rounded-full bg-border"></span>
			<span class="ml-3 text-xs font-medium text-muted-foreground">{m['vms.list.calmHeading']()}</span>
			<span class="ml-auto rounded-full bg-primary/10 px-2 py-0.5 text-[0.6875rem] font-medium text-primary">3 / 5</span>
		</div>
		<ul class="divide-y divide-border">
			{#each rows as row (row.name)}
				<li class="flex items-center gap-3 px-4 py-3">
					<span class="grid h-8 w-8 shrink-0 place-items-center rounded-md bg-primary/10 text-[0.625rem] font-semibold text-primary">
						VM
					</span>
					<div class="min-w-0 flex-1">
						<p class="truncate font-mono text-sm text-foreground">{row.name}</p>
						<p class="text-xs text-muted-foreground-subtle">{row.node}</p>
					</div>
					<div class="hidden w-20 sm:block">
						<div class="h-1.5 overflow-hidden rounded-full bg-muted">
							{#if row.state !== 'stopped'}
								<div class="pv-load h-full rounded-full bg-primary/70 {row.load}" style:animation-duration={row.speed}></div>
							{/if}
						</div>
					</div>
					<span class="inline-flex w-24 items-center gap-1.5 text-xs text-muted-foreground">
						{#if row.state === 'running'}
							<span class="h-2 w-2 rounded-full bg-success"></span>{m['common.running']()}
						{:else if row.state === 'stopped'}
							<span class="h-2 w-2 rounded-full bg-muted-foreground"></span>{m['common.stopped']()}
						{:else}
							<span class="pv-boot-dot h-2 w-2 rounded-full bg-success"></span>
							<span class="relative">
								<span class="pv-boot-label-off absolute left-0 opacity-0">{m['machine.status.starting']()}</span>
								<span class="pv-boot-label-on">{m['common.running']()}</span>
							</span>
						{/if}
					</span>
				</li>
			{/each}
		</ul>
	</div>
</div>
