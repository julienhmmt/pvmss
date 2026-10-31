<script lang="ts">
	import { getVmCreateContext, memoryLabel } from '../create.svelte';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Switch from '#lib/shared/ui/Switch.svelte';
	import { m } from '#lib/paraglide/messages.js';

	// Review step (V09): shows the exact request the server will receive -
	// there is no second, expert-only endpoint (FR-001) and no hidden fields.
	// The readable summary is derived from that same request, so the two
	// views can never disagree; the raw JSON stays one switch away.
	const form = getVmCreateContext();

	const outgoing = $derived(form.buildRequest());
	let showJson = $state(false);

	interface Row {
		label: string;
		value: string;
		mono?: boolean;
	}

	const sections = $derived.by((): { title: string; rows: Row[] }[] => {
		const req = outgoing;
		const cat = form.catalog;
		const template = req.templateId !== undefined ? cat?.templates.find((t) => t.vmid === req.templateId) : undefined;
		const profile = req.profileId !== undefined ? cat?.profiles.find((p) => p.id === req.profileId) : undefined;

		const source: Row = req.templateId !== undefined
			? { label: m['vms.create.sourceTemplate'](), value: template?.name ?? String(req.templateId), mono: true }
			: { label: m['vms.create.sourceIso'](), value: req.iso?.file ?? m['vms.create.summary.none'](), mono: req.iso !== undefined };
		const node = req.node ?? (template ? m['vms.create.review.nodeFromTemplate']({ node: template.node }) : m['common.dash']());

		// A profile overrides the explicit hardware fields server-side
		// (FR-009), so show what will actually be applied.
		const hardware: Row[] = profile
			? [{ label: m['vms.create.profile'](), value: m['vms.create.review.profileSpec']({ label: profile.label, sockets: profile.sockets, cores: profile.cpuCores, memory: memoryLabel(profile.memoryMB) }) }]
			: [
					{ label: m['vms.create.sockets'](), value: String(req.sockets ?? m['common.dash']()) },
					{ label: m['vms.create.vcpuCores'](), value: String(req.cpuCores ?? m['common.dash']()) },
					{ label: m['vms.create.memory'](), value: req.memoryMB !== undefined ? memoryLabel(req.memoryMB) : m['common.dash']() }
				];

		const options: Row[] = [
			{ label: m['vms.create.firmwareGroup'](), value: req.uefi === false ? m['vms.create.review.bios']() : req.tpm ? `${m['vms.create.uefi']()} + ${m['vms.create.tpm']()}` : m['vms.create.uefi']() },
			{ label: m['vms.create.startAfterCreate'](), value: req.startAfterCreate ? m['common.yes']() : m['common.no']() }
		];

		return [
			{
				title: m['vms.create.review.sectionMachine'](),
				rows: [
					{ label: m['vms.create.name'](), value: req.name, mono: true },
					{ label: m['vms.create.review.cluster'](), value: form.clusterDisplayName() },
					{ label: m['vms.create.node'](), value: node, mono: req.node !== undefined },
					source,
					{ label: m['vms.create.tags'](), value: req.tags?.join(', ') ?? m['vms.create.summary.none']() }
				]
			},
			{ title: m['vms.create.stepHardware'](), rows: hardware },
			{
				title: m['vms.create.stepDisk'](),
				rows: [
					{ label: m['vms.create.storage'](), value: req.disk?.storage ?? m['common.dash'](), mono: true },
					{ label: m['vms.create.size'](), value: req.disk?.sizeGB !== undefined ? `${req.disk.sizeGB} GB` : m['common.dash']() }
				]
			},
			{
				title: m['vms.create.stepNetwork'](),
				rows: (req.network ?? []).map((nic, index) => ({ label: m['vms.create.nicLabel']({ index }), value: `${nic.bridge || m['common.dash']()} · ${nic.model}`, mono: true }))
			},
			{ title: m['vms.create.review.sectionOptions'](), rows: options }
		];
	});
</script>

<div class="grid gap-5">
	<div class="flex flex-wrap items-center justify-between gap-3">
		<h2 class="text-base font-semibold">{m['vms.create.reviewHeading']()}</h2>
		<label class="inline-flex cursor-pointer items-center gap-2 text-sm text-muted-foreground">
			{m['vms.create.review.showJson']()}
			<Switch label={m['vms.create.review.showJson']()} checked={showJson} onToggle={() => (showJson = !showJson)} />
		</label>
	</div>
	{#if showJson}
		<div class="grid gap-2">
			<p class="text-xs text-muted-foreground">{m['vms.create.review.jsonHint']()}</p>
			<pre
				class="overflow-x-auto rounded-lg border border-border bg-muted p-4 font-mono text-xs"
				data-testid="review-request">{JSON.stringify(outgoing, null, 2)}</pre>
		</div>
	{:else}
		<div class="grid gap-3 sm:grid-cols-2" data-testid="review-summary">
			{#each sections as section (section.title)}
				<section class="rounded-lg border border-border bg-muted/40 p-4">
					<h3 class="mb-2.5 text-xs font-semibold tracking-wide text-muted-foreground uppercase">{section.title}</h3>
					<dl class="grid grid-cols-[auto_minmax(0,1fr)] gap-x-4 gap-y-1.5 text-sm">
						{#each section.rows as row (row.label)}
							<dt class="text-muted-foreground">{row.label}</dt>
							<dd class="text-right font-medium [overflow-wrap:anywhere] {row.mono ? 'font-mono text-xs leading-5' : ''}">{row.value}</dd>
						{/each}
					</dl>
				</section>
			{/each}
		</div>
	{/if}

	{#if form.submitError}<Alert>{form.submitError}</Alert>{/if}
</div>
