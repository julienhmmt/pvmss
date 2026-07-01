<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Checkbox } from '$lib/components/ui/checkbox';
	import * as Card from '$lib/components/ui/card';
	import { Switch } from '$lib/components/ui/switch';
	import {
		CheckCircle,
		XCircle,
		CaretRight,
		CaretLeft,
		SpinnerGap,
		Desktop,
		HardDrive,
		WifiHigh,
		Check,
		GearSix,
	} from 'phosphor-svelte';
	import {
		getSetupStatus,
		testConnection,
		getProxmoxData,
		completeSetup,
		type SetupStatus,
		type SetupConnectionTestResult,
		type SetupProxmoxData,
		type SetupLimits,
	} from '$lib/api/setup';

	type Step = 'connection' | 'nodes' | 'resources' | 'limits' | 'review' | 'done';

	const STEPS: Step[] = ['connection', 'nodes', 'resources', 'limits', 'review'];

	let currentStep = $state<Step>('connection');
	let loading = $state(true);
	let error = $state<string | null>(null);

	let status = $state<SetupStatus | null>(null);
	let connectionResult = $state<SetupConnectionTestResult | null>(null);
	let connectionTesting = $state(false);
	let proxmoxData = $state<SetupProxmoxData | null>(null);
	let dataLoading = $state(false);

	let selectedNodes = $state<Set<string>>(new Set());
	let selectedStorages = $state<Set<string>>(new Set());
	let selectedISOs = $state<Set<string>>(new Set());
	let selectedVMBRs = $state<Set<string>>(new Set());

	let limits = $state<SetupLimits>({
		maxVms: 20,
		maxVmPerUser: 5,
		maxNetworkCards: 2,
		maxDiskPerVm: 4,
		maxSnapshots: 5,
		allowCustomYaml: false,
	});

	let completing = $state(false);

	const stepIndex = $derived(STEPS.indexOf(currentStep as Step));

	onMount(async () => {
		try {
			status = await getSetupStatus();
			if (status.complete) {
				window.location.href = '/login';
				return;
			}
			if (status.offline) {
				connectionResult = { ok: false, proxmoxUrl: '', error: 'offline' };
			}
		} catch (err: unknown) {
			console.error('getSetupStatus failed:', err instanceof Error ? err.message : String(err));
			error = 'Failed to load setup status.';
		} finally {
			loading = false;
		}
	});

	async function handleTestConnection() {
		connectionTesting = true;
		connectionResult = null;
		try {
			connectionResult = await testConnection();
		} catch (err: unknown) {
			console.error('testConnection failed:', err instanceof Error ? err.message : String(err));
			connectionResult = { ok: false, proxmoxUrl: '', error: 'Network error' };
		} finally {
			connectionTesting = false;
		}
	}

	async function loadProxmoxData() {
		if (proxmoxData) return;
		dataLoading = true;
		try {
			proxmoxData = await getProxmoxData();
			selectedNodes = new Set(proxmoxData.nodes);
			selectedStorages = new Set(proxmoxData.storages);
			selectedISOs = new Set(proxmoxData.isos);
			selectedVMBRs = new Set(proxmoxData.vmbrs);
		} catch (err: unknown) {
			console.error('getProxmoxData failed:', err instanceof Error ? err.message : String(err));
			proxmoxData = { nodes: [], storages: [], isos: [], vmbrs: [] };
		} finally {
			dataLoading = false;
		}
	}

	function toggleItem(set: Set<string>, item: string): Set<string> {
		const next = new Set(set);
		if (next.has(item)) next.delete(item);
		else next.add(item);
		return next;
	}

	async function goNext() {
		if (currentStep === 'connection') {
			if (!connectionResult) await handleTestConnection();
			await loadProxmoxData();
			currentStep = 'nodes';
		} else if (currentStep === 'nodes') {
			currentStep = 'resources';
		} else if (currentStep === 'resources') {
			currentStep = 'limits';
		} else if (currentStep === 'limits') {
			currentStep = 'review';
		} else if (currentStep === 'review') {
			await handleComplete();
		}
	}

	function goBack() {
		if (currentStep === 'nodes') currentStep = 'connection';
		else if (currentStep === 'resources') currentStep = 'nodes';
		else if (currentStep === 'limits') currentStep = 'resources';
		else if (currentStep === 'review') currentStep = 'limits';
	}

	async function handleComplete() {
		completing = true;
		error = null;
		try {
			await completeSetup({
				enabledNodes: [...selectedNodes],
				enabledStorages: [...selectedStorages],
				enabledIsos: [...selectedISOs],
				enabledVmbrs: [...selectedVMBRs],
				limits,
			});
			currentStep = 'done';
		} catch (err: unknown) {
			console.error('completeSetup failed:', err instanceof Error ? err.message : String(err));
			error = 'Failed to complete setup. Please try again.';
		} finally {
			completing = false;
		}
	}
</script>

<svelte:head>
	<title>{$t('setup.title')} — PVMSS</title>
</svelte:head>

<div class="min-h-screen bg-background flex items-center justify-center p-4">
	<div class="w-full max-w-2xl">

		{#if loading}
			<div class="flex items-center justify-center py-16">
				<SpinnerGap class="animate-spin h-8 w-8 text-muted-foreground" />
			</div>

		{:else if currentStep === 'done'}
			<Card.Root>
				<Card.Content class="flex flex-col items-center gap-4 py-12">
					<CheckCircle class="h-16 w-16 text-success" />
					<h1 class="text-2xl font-bold">{$t('setup.done.title')}</h1>
					<p class="text-muted-foreground text-center">{$t('setup.done.description')}</p>
					<Button href="/login" class="mt-4">{$t('setup.done.login')}</Button>
				</Card.Content>
			</Card.Root>

		{:else}
			<!-- Step indicator -->
			<div class="mb-6 flex items-center gap-2">
				{#each STEPS as step, i (i)}
					<div class="flex items-center gap-2 flex-1">
						<div class="flex flex-col items-center gap-1">
							<div class="flex items-center justify-center h-7 w-7 rounded-full text-xs font-medium
								{i < stepIndex ? 'bg-primary text-primary-foreground'
								: i === stepIndex ? 'bg-primary text-primary-foreground ring-2 ring-primary/30'
								: 'bg-muted text-muted-foreground'}">
								{#if i < stepIndex}
									<Check class="h-4 w-4" />
								{:else}
									{i + 1}
								{/if}
							</div>
							<span class="text-[10px] text-muted-foreground hidden sm:block whitespace-nowrap">
								{$t(`setup.steps.${step}`)}
							</span>
						</div>
						{#if i < STEPS.length - 1}
							<div class="h-px flex-1 {i < stepIndex ? 'bg-primary' : 'bg-border'} mb-4"></div>
						{/if}
					</div>
				{/each}
			</div>

			<Card.Root>
				<Card.Header>
					<Card.Title>{$t(`setup.${currentStep}.title`)}</Card.Title>
					<Card.Description>{$t(`setup.${currentStep}.description`)}</Card.Description>
				</Card.Header>

				<Card.Content class="space-y-4">

					<!-- Step 1: Connection -->
					{#if currentStep === 'connection'}
						{#if status?.offline}
							<div class="rounded-lg bg-warning-soft border border-warning-soft-border p-4 space-y-1">
								<p class="font-medium text-warning-soft-foreground">{$t('setup.connection.offlineTitle')}</p>
								<p class="text-sm text-warning-soft-foreground/80">{$t('setup.connection.offlineDesc')}</p>
							</div>
						{:else}
							<div class="space-y-2">
								<Label>{$t('setup.connection.url')}</Label>
								<Input value={status?.proxmoxUrl ?? ''} readonly class="font-mono text-sm" />
							</div>

							<Button
								onclick={handleTestConnection}
								disabled={connectionTesting}
								variant="outline"
								class="w-full"
							>
								{#if connectionTesting}
									<SpinnerGap class="animate-spin h-4 w-4 mr-2" />
									{$t('setup.connection.testing')}
								{:else}
									{$t('setup.connection.retry')}
								{/if}
							</Button>

							{#if connectionResult}
								<div class="flex items-center gap-2 rounded-lg p-3
									{connectionResult.ok
										? 'bg-success-soft border border-success-soft-border'
										: 'bg-destructive-soft border border-destructive-soft-border'}">
									{#if connectionResult.ok}
										<CheckCircle class="h-5 w-5 text-success shrink-0" />
										<span class="text-sm text-success">{$t('setup.connection.success')}</span>
									{:else}
										<XCircle class="h-5 w-5 text-destructive shrink-0" />
										<span class="text-sm text-destructive-soft-foreground">
											{$t('setup.connection.failure')}
											{#if connectionResult.error && connectionResult.error !== 'offline'}
												— {connectionResult.error}
											{/if}
										</span>
									{/if}
								</div>
							{/if}
						{/if}

					<!-- Step 2: Nodes -->
					{:else if currentStep === 'nodes'}
						{#if dataLoading}
							<div class="flex justify-center py-8">
								<SpinnerGap class="animate-spin h-6 w-6 text-muted-foreground" />
							</div>
						{:else if !proxmoxData || proxmoxData.nodes.length === 0}
							<p class="text-sm text-muted-foreground">{$t('setup.nodes.none')}</p>
						{:else}
							<div class="flex gap-2 mb-2">
								<Button variant="ghost" size="sm" onclick={() => { selectedNodes = new Set(proxmoxData!.nodes); }}>
									{$t('setup.nodes.selectAll')}
								</Button>
								<Button variant="ghost" size="sm" onclick={() => { selectedNodes = new Set(); }}>
									{$t('setup.nodes.deselectAll')}
								</Button>
							</div>
							<div class="space-y-2">
								{#each proxmoxData.nodes as node, i (i)}
									<div class="flex items-center gap-3 p-3 rounded-lg border">
										<Checkbox
											id="node-{node}"
											checked={selectedNodes.has(node)}
											onCheckedChange={() => { selectedNodes = toggleItem(selectedNodes, node); }}
										/>
										<Label for="node-{node}" class="flex items-center gap-2 cursor-pointer">
											<Desktop class="h-4 w-4 text-muted-foreground" />
											{node}
										</Label>
									</div>
								{/each}
							</div>
						{/if}

					<!-- Step 3: Resources (storages / ISOs / bridges) -->
					{:else if currentStep === 'resources'}
						{#if dataLoading}
							<div class="flex justify-center py-8">
								<SpinnerGap class="animate-spin h-6 w-6 text-muted-foreground" />
							</div>
						{:else}
							<!-- Storages -->
							<div class="space-y-2">
								<div>
									<h3 class="font-medium flex items-center gap-2">
										<HardDrive class="h-4 w-4 text-muted-foreground" />
										{$t('setup.resources.storagesTitle')}
									</h3>
									<p class="text-xs text-muted-foreground">{$t('setup.resources.storagesDesc')}</p>
								</div>
								{#if (proxmoxData?.storages ?? []).length === 0}
									<p class="text-sm text-muted-foreground italic">{$t('setup.resources.none')}</p>
								{:else}
									<div class="flex gap-2 mb-1">
										<Button variant="ghost" size="sm" onclick={() => { selectedStorages = new Set(proxmoxData!.storages); }}>{$t('setup.resources.selectAll')}</Button>
										<Button variant="ghost" size="sm" onclick={() => { selectedStorages = new Set(); }}>{$t('setup.resources.deselectAll')}</Button>
									</div>
									<div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
										{#each proxmoxData?.storages ?? [] as item, i (i)}
											<div class="flex items-center gap-2 p-2 rounded border text-sm">
												<Checkbox id="storage-{item}" checked={selectedStorages.has(item)} onCheckedChange={() => { selectedStorages = toggleItem(selectedStorages, item); }} />
												<Label for="storage-{item}" class="cursor-pointer truncate">{item}</Label>
											</div>
										{/each}
									</div>
								{/if}
							</div>
							<hr class="border-border" />
							<!-- ISOs -->
							<div class="space-y-2">
								<div>
									<h3 class="font-medium flex items-center gap-2">
										<Desktop class="h-4 w-4 text-muted-foreground" />
										{$t('setup.resources.isosTitle')}
									</h3>
									<p class="text-xs text-muted-foreground">{$t('setup.resources.isosDesc')}</p>
								</div>
								{#if (proxmoxData?.isos ?? []).length === 0}
									<p class="text-sm text-muted-foreground italic">{$t('setup.resources.none')}</p>
								{:else}
									<div class="flex gap-2 mb-1">
										<Button variant="ghost" size="sm" onclick={() => { selectedISOs = new Set(proxmoxData!.isos); }}>{$t('setup.resources.selectAll')}</Button>
										<Button variant="ghost" size="sm" onclick={() => { selectedISOs = new Set(); }}>{$t('setup.resources.deselectAll')}</Button>
									</div>
									<div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
										{#each proxmoxData?.isos ?? [] as item, i (i)}
											<div class="flex items-center gap-2 p-2 rounded border text-sm">
												<Checkbox id="iso-{item}" checked={selectedISOs.has(item)} onCheckedChange={() => { selectedISOs = toggleItem(selectedISOs, item); }} />
												<Label for="iso-{item}" class="cursor-pointer truncate">{item}</Label>
											</div>
										{/each}
									</div>
								{/if}
							</div>
							<hr class="border-border" />
							<!-- VMBRs -->
							<div class="space-y-2">
								<div>
									<h3 class="font-medium flex items-center gap-2">
										<WifiHigh class="h-4 w-4 text-muted-foreground" />
										{$t('setup.resources.vmbrsTitle')}
									</h3>
									<p class="text-xs text-muted-foreground">{$t('setup.resources.vmbrsDesc')}</p>
								</div>
								{#if (proxmoxData?.vmbrs ?? []).length === 0}
									<p class="text-sm text-muted-foreground italic">{$t('setup.resources.none')}</p>
								{:else}
									<div class="flex gap-2 mb-1">
										<Button variant="ghost" size="sm" onclick={() => { selectedVMBRs = new Set(proxmoxData!.vmbrs); }}>{$t('setup.resources.selectAll')}</Button>
										<Button variant="ghost" size="sm" onclick={() => { selectedVMBRs = new Set(); }}>{$t('setup.resources.deselectAll')}</Button>
									</div>
									<div class="grid grid-cols-1 sm:grid-cols-2 gap-2">
										{#each proxmoxData?.vmbrs ?? [] as item, i (i)}
											<div class="flex items-center gap-2 p-2 rounded border text-sm">
												<Checkbox id="vmbr-{item}" checked={selectedVMBRs.has(item)} onCheckedChange={() => { selectedVMBRs = toggleItem(selectedVMBRs, item); }} />
												<Label for="vmbr-{item}" class="cursor-pointer truncate">{item}</Label>
											</div>
										{/each}
									</div>
								{/if}
							</div>
						{/if}

					<!-- Step 4: Limits -->
					{:else if currentStep === 'limits'}
						<p class="text-sm text-muted-foreground">{$t('setup.limits.description')}</p>
						<div class="grid grid-cols-1 sm:grid-cols-2 gap-4">
							{#each [
								{ key: 'maxVms', label: $t('setup.limits.maxVms') },
								{ key: 'maxVmPerUser', label: $t('setup.limits.maxVmPerUser') },
								{ key: 'maxNetworkCards', label: $t('setup.limits.maxNetworkCards') },
								{ key: 'maxDiskPerVm', label: $t('setup.limits.maxDiskPerVM') },
								{ key: 'maxSnapshots', label: $t('setup.limits.maxSnapshots') },
							] as field, i (i)}
								<div class="space-y-1">
									<Label for="limit-{field.key}">{field.label}</Label>
									<Input
										id="limit-{field.key}"
										type="number"
										min="1"
										value={limits[field.key as keyof SetupLimits] as number}
										oninput={(e) => {
											const v = parseInt((e.target as HTMLInputElement).value, 10);
											if (!isNaN(v) && v > 0) limits = { ...limits, [field.key]: v };
										}}
									/>
								</div>
							{/each}
							<div class="flex items-center gap-3 sm:col-span-2 pt-1">
								<Switch
									id="allow-yaml"
									checked={limits.allowCustomYaml}
									onCheckedChange={(v) => { limits = { ...limits, allowCustomYaml: v }; }}
								/>
								<Label for="allow-yaml">{$t('setup.limits.allowCustomYaml')}</Label>
							</div>
						</div>

					<!-- Step 5: Review -->
					{:else if currentStep === 'review'}
						<div class="space-y-4 text-sm">
							{#each [
								{ label: $t('setup.review.nodes'), items: [...selectedNodes] },
								{ label: $t('setup.review.storages'), items: [...selectedStorages] },
								{ label: $t('setup.review.isos'), items: [...selectedISOs] },
								{ label: $t('setup.review.vmbrs'), items: [...selectedVMBRs] },
							] as section, i (i)}
								<div>
									<span class="font-medium">{section.label}:</span>
									{#if section.items.length === 0}
										<span class="text-muted-foreground ml-1">{$t('setup.review.none')}</span>
									{:else}
										<span class="ml-1">{section.items.join(', ')}</span>
									{/if}
								</div>
							{/each}
							<div>
								<span class="font-medium">{$t('setup.review.limits')}:</span>
								<ul class="mt-1 ml-4 space-y-0.5 text-muted-foreground">
									<li>{$t('setup.limits.maxVms')}: {limits.maxVms}</li>
									<li>{$t('setup.limits.maxVmPerUser')}: {limits.maxVmPerUser}</li>
									<li>{$t('setup.limits.maxNetworkCards')}: {limits.maxNetworkCards}</li>
									<li>{$t('setup.limits.maxDiskPerVM')}: {limits.maxDiskPerVm}</li>
									<li>{$t('setup.limits.maxSnapshots')}: {limits.maxSnapshots}</li>
								</ul>
							</div>
						</div>
						{#if error}
							<div class="rounded-lg bg-destructive-soft border border-destructive-soft-border p-3">
								<p class="text-sm text-destructive-soft-foreground">{error}</p>
							</div>
						{/if}
					{/if}

				</Card.Content>

				<Card.Footer class="flex justify-between">
					<Button
						variant="outline"
						onclick={goBack}
						disabled={currentStep === 'connection' || completing}
					>
						<CaretLeft class="h-4 w-4 mr-2" />
						{$t('setup.nav.back')}
					</Button>
					<Button
						onclick={goNext}
						disabled={completing}
					>
						{#if completing}
							<SpinnerGap class="animate-spin h-4 w-4 mr-2" />
							{$t('setup.review.completing')}
						{:else if currentStep === 'review'}
							<GearSix class="h-4 w-4 mr-2" />
							{$t('setup.review.complete')}
						{:else}
							{$t('setup.nav.next')}
							<CaretRight class="h-4 w-4 ml-2" />
						{/if}
					</Button>
				</Card.Footer>
			</Card.Root>
		{/if}

	</div>
</div>
