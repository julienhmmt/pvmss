<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Tabs from '$lib/components/ui/tabs';
	import { getLimits, updateLimits } from '$lib/api/admin/limits';
	import { Sliders } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Limits, ResourceRange } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let saving = $state(false);
	let limits = $state<Limits | null>(null);

	let nodeNames = $derived(limits ? Object.keys(limits.nodes).sort() : []);
	let selectedNode = $state('');

	async function load() {
		loading = true;
		error = null;
		try {
			limits = await getLimits();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
		}
	}

	function coerceRange(r: ResourceRange): ResourceRange {
		return { min: Number(r.min), max: Number(r.max) };
	}

	function coerceLimits(l: Limits): Limits {
		const nodes: Record<string, { sockets: ResourceRange; cores: ResourceRange; ram: ResourceRange; disk: ResourceRange }> = {};
		for (const [k, v] of Object.entries(l.nodes)) {
			nodes[k] = {
				sockets: coerceRange(v.sockets),
				cores: coerceRange(v.cores),
				ram: coerceRange(v.ram),
				disk: coerceRange(v.disk)
			};
		}
		return {
			vm: {
				sockets: coerceRange(l.vm.sockets),
				cores: coerceRange(l.vm.cores),
				ram: coerceRange(l.vm.ram),
				disk: coerceRange(l.vm.disk)
			},
			nodes,
			max_snapshots: Number(l.max_snapshots),
			max_network_cards: Number(l.max_network_cards),
			max_disk_per_vm: Number(l.max_disk_per_vm),
			max_vm_per_user: Number(l.max_vm_per_user)
		};
	}

	async function save() {
		if (!limits) return;
		saving = true;
		try {
			await updateLimits(coerceLimits(limits));
			toast.success($t('admin.limits.toast.saved'));
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			saving = false;
		}
	}

	onMount(load);
</script>

<PageHeader title={$t('admin.limits.title')} icon={Sliders}>
	{#snippet actions()}
		<Button onclick={save} disabled={saving || !limits}>
			{saving ? $t('common.saving') : $t('common.save')}
		</Button>
	{/snippet}
</PageHeader>

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading || !limits}
	<LoadingSkeleton variant="form" rows={6} />
{:else}
	<div class="max-w-2xl space-y-8">
		<section class="space-y-4">
			<h2 class="text-lg font-semibold">{$t('admin.limits.vmResourceRanges')}</h2>
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label>{$t('admin.limits.cpuSocketsMin')}</Label>
					<Input type="number" bind:value={limits.vm.sockets.min} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.cpuSocketsMax')}</Label>
					<Input type="number" bind:value={limits.vm.sockets.max} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.cpuCoresMin')}</Label>
					<Input type="number" bind:value={limits.vm.cores.min} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.cpuCoresMax')}</Label>
					<Input type="number" bind:value={limits.vm.cores.max} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.ramGbMin')}</Label>
					<Input type="number" bind:value={limits.vm.ram.min} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.ramGbMax')}</Label>
					<Input type="number" bind:value={limits.vm.ram.max} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.diskGbMin')}</Label>
					<Input type="number" bind:value={limits.vm.disk.min} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.diskGbMax')}</Label>
					<Input type="number" bind:value={limits.vm.disk.max} />
				</div>
			</div>
		</section>

		<section class="space-y-4">
			<h2 class="text-lg font-semibold">{$t('admin.limits.globalLimits')}</h2>
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label>{$t('admin.limits.maxVmsPerUser')}</Label>
					<Input type="number" bind:value={limits.max_vm_per_user} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.maxSnapshots')}</Label>
					<Input type="number" bind:value={limits.max_snapshots} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.maxNetworkCards')}</Label>
					<Input type="number" bind:value={limits.max_network_cards} />
				</div>
				<div class="space-y-2">
					<Label>{$t('admin.limits.maxDisksPerVm')}</Label>
					<Input type="number" bind:value={limits.max_disk_per_vm} />
				</div>
			</div>
		</section>

		<section class="space-y-4">
			<h2 class="text-lg font-semibold">{$t('admin.limits.nodeSpecificLimits')}</h2>
			{#if nodeNames.length > 0}
				<Tabs.Root value={selectedNode || nodeNames[0]} onValueChange={(v) => (selectedNode = v)}>
					<Tabs.List>
						{#each nodeNames as name}
							<Tabs.Trigger value={name}>{name}</Tabs.Trigger>
						{/each}
					</Tabs.List>
					{#each nodeNames as name}
						<Tabs.Content value={name}>
							<div class="grid grid-cols-2 gap-4 pt-4">
								<div class="space-y-2">
									<Label>{$t('admin.limits.cpuSocketsMin')}</Label>
									<Input type="number" bind:value={limits.nodes[name].sockets.min} />
								</div>
								<div class="space-y-2">
									<Label>{$t('admin.limits.cpuSocketsMax')}</Label>
									<Input type="number" bind:value={limits.nodes[name].sockets.max} />
								</div>
								<div class="space-y-2">
									<Label>{$t('admin.limits.cpuCoresMin')}</Label>
									<Input type="number" bind:value={limits.nodes[name].cores.min} />
								</div>
								<div class="space-y-2">
									<Label>{$t('admin.limits.cpuCoresMax')}</Label>
									<Input type="number" bind:value={limits.nodes[name].cores.max} />
								</div>
								<div class="space-y-2">
									<Label>{$t('admin.limits.ramMin')}</Label>
									<Input type="number" bind:value={limits.nodes[name].ram.min} />
								</div>
								<div class="space-y-2">
									<Label>{$t('admin.limits.ramMax')}</Label>
									<Input type="number" bind:value={limits.nodes[name].ram.max} />
								</div>
								<div class="space-y-2">
									<Label>{$t('admin.limits.diskMin')}</Label>
									<Input type="number" bind:value={limits.nodes[name].disk.min} />
								</div>
								<div class="space-y-2">
									<Label>{$t('admin.limits.diskMax')}</Label>
									<Input type="number" bind:value={limits.nodes[name].disk.max} />
								</div>
							</div>
						</Tabs.Content>
					{/each}
				</Tabs.Root>
			{:else}
					<p class="text-sm text-muted-foreground">
						{$t('admin.limits.noNodeLimits')}
					</p>
			{/if}
		</section>
	</div>
{/if}
