<script lang="ts">
	import { onMount } from 'svelte';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { getLimits, updateLimits } from '$lib/api/admin/limits';
	import { Sliders } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { Limits, ResourceRange } from '$lib/types/admin';

	let loading = $state(true);
	let error = $state<Error | null>(null);
	let saving = $state(false);
	let limits = $state<Limits | null>(null);

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
			toast.success('Limits updated');
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			saving = false;
		}
	}

	onMount(load);
</script>

<PageHeader title="Resource Limits" icon={Sliders}>
	{#snippet actions()}
		<Button onclick={save} disabled={saving || !limits}>
			{saving ? 'Saving...' : 'Save'}
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
			<h2 class="text-lg font-semibold">VM Resource Ranges</h2>
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label>CPU Sockets (min)</Label>
					<Input type="number" bind:value={limits.vm.sockets.min} />
				</div>
				<div class="space-y-2">
					<Label>CPU Sockets (max)</Label>
					<Input type="number" bind:value={limits.vm.sockets.max} />
				</div>
				<div class="space-y-2">
					<Label>CPU Cores (min)</Label>
					<Input type="number" bind:value={limits.vm.cores.min} />
				</div>
				<div class="space-y-2">
					<Label>CPU Cores (max)</Label>
					<Input type="number" bind:value={limits.vm.cores.max} />
				</div>
				<div class="space-y-2">
					<Label>RAM GB (min)</Label>
					<Input type="number" bind:value={limits.vm.ram.min} />
				</div>
				<div class="space-y-2">
					<Label>RAM GB (max)</Label>
					<Input type="number" bind:value={limits.vm.ram.max} />
				</div>
				<div class="space-y-2">
					<Label>Disk GB (min)</Label>
					<Input type="number" bind:value={limits.vm.disk.min} />
				</div>
				<div class="space-y-2">
					<Label>Disk GB (max)</Label>
					<Input type="number" bind:value={limits.vm.disk.max} />
				</div>
			</div>
		</section>

		<section class="space-y-4">
			<h2 class="text-lg font-semibold">Global Limits</h2>
			<div class="grid grid-cols-2 gap-4">
				<div class="space-y-2">
					<Label>Max VMs per User</Label>
					<Input type="number" bind:value={limits.max_vm_per_user} />
				</div>
				<div class="space-y-2">
					<Label>Max Snapshots per VM</Label>
					<Input type="number" bind:value={limits.max_snapshots} />
				</div>
				<div class="space-y-2">
					<Label>Max Network Cards</Label>
					<Input type="number" bind:value={limits.max_network_cards} />
				</div>
				<div class="space-y-2">
					<Label>Max Disks per VM</Label>
					<Input type="number" bind:value={limits.max_disk_per_vm} />
				</div>
			</div>
		</section>
	</div>
{/if}
