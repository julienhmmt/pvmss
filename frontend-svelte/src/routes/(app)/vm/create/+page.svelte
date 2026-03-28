<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { getVMCreateSettings, createVM } from '$lib/api/vm-create';
	import type {
		VMCreateSettings,
		VMCreateRequest,
		VMCreateDisk,
		VMCreateNetwork,
		VMCreateCloudInit
	} from '$lib/types/vm-create';
	import { NETWORK_MODELS, DISK_BUSES } from '$lib/types/vm-create';
	import { Card, CardContent, CardHeader, CardTitle } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Switch } from '$lib/components/ui/switch';
	import { Separator } from '$lib/components/ui/separator';
	import { Badge } from '$lib/components/ui/badge';
	import * as Select from '$lib/components/ui/select';
	import { Textarea } from '$lib/components/ui/textarea';
	import {
		Monitor,
		Cpu,
		HardDrive,
		WifiHigh,
		Cloud,
		CheckCircle,
		CaretLeft,
		CaretRight,
		Plus,
		Trash,
		SpinnerGap,
		Warning,
		Check
	} from 'phosphor-svelte';

	const STEPS = ['base', 'hardware', 'disk', 'network', 'cloudinit', 'review'] as const;
	type Step = (typeof STEPS)[number];

	const STEP_ICONS: Record<Step, typeof Monitor> = {
		base: Monitor,
		hardware: Cpu,
		disk: HardDrive,
		network: WifiHigh,
		cloudinit: Cloud,
		review: CheckCircle
	};

	let settings: VMCreateSettings | null = $state(null);
	let loading: boolean = $state(true);
	let creating: boolean = $state(false);
	let currentStep: number = $state(0);

	// Form state
	let vmName: string = $state('');
	let vmNode: string = $state('');
	let vmStorage: string = $state('');
	let vmISO: string = $state('');
	let vmDescription: string = $state('');
	let vmTags: string[] = $state([]);

	let vmSockets: number = $state(1);
	let vmCores: number = $state(1);
	let vmMemoryGB: number = $state(1);
	let vmDiskBus: string = $state('virtio');
	let vmEnableEFI: boolean = $state(false);
	let vmEnableTPM: boolean = $state(false);

	let vmDisks: VMCreateDisk[] = $state([{ size_gb: 10 }]);
	let vmNetworks: VMCreateNetwork[] = $state([
		{ bridge: '', model: 'virtio', mac: '', vlan: 0, rate_limit: '', mtu: 0, enabled: true }
	]);

	let vmCloudInitEnabled: boolean = $state(false);
	let vmCIUser: string = $state('');
	let vmCIPassword: string = $state('');
	let vmCISSHKeys: string = $state('');
	let vmCIIPConfig: string = $state('dhcp');
	let vmCIIP: string = $state('');
	let vmCIGateway: string = $state('');
	let vmCIDNS: string = $state('');
	let vmCITemplateID: string = $state('');

	let vmStartAfterCreation: boolean = $state(true);

	const currentStepName = $derived(STEPS[currentStep]);
	const isFirstStep = $derived(currentStep === 0);
	const isLastStep = $derived(currentStep === STEPS.length - 1);
	const totalVCPUs = $derived(vmSockets * vmCores);
	const totalDiskGB = $derived(vmDisks.reduce((acc, d) => acc + d.size_gb, 0));

	const quotaBlocked = $derived(
		settings !== null && settings.remaining_vms !== -1 && settings.remaining_vms <= 0
	);

	onMount(async () => {
		try {
			settings = await getVMCreateSettings();
			applyDefaults();
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			toast.error(msg);
		} finally {
			loading = false;
		}
	});

	function applyDefaults(): void {
		if (!settings) return;
		vmSockets = settings.limits.sockets.min;
		vmCores = settings.limits.cores.min;
		vmMemoryGB = settings.limits.ram.min;
		vmDisks = [{ size_gb: settings.limits.disk.min }];
		if (settings.nodes.length > 0) {
			const firstEnabled = settings.nodes.find((n) => !n.disabled);
			if (firstEnabled) vmNode = firstEnabled.name;
		}
		if (settings.storages.length > 0) {
			vmStorage = settings.storages[0].name;
		}
		if (settings.bridges.length > 0) {
			vmNetworks = [
				{
					bridge: settings.bridges[0].name,
					model: 'virtio',
					mac: '',
					vlan: 0,
					rate_limit: '',
					mtu: 0,
					enabled: true
				}
			];
		}
	}

	function goNext(): void {
		if (currentStep < STEPS.length - 1) currentStep++;
	}

	function goPrev(): void {
		if (currentStep > 0) currentStep--;
	}

	function goToStep(index: number): void {
		if (index >= 0 && index < STEPS.length) currentStep = index;
	}

	function addDisk(): void {
		if (!settings) return;
		if (vmDisks.length >= settings.max_disk_per_vm) return;
		vmDisks = [...vmDisks, { size_gb: settings.limits.disk.min }];
	}

	function removeDisk(index: number): void {
		if (index === 0 || vmDisks.length <= 1) return;
		vmDisks = vmDisks.filter((_, i) => i !== index);
	}

	function addNetworkCard(): void {
		if (!settings) return;
		if (vmNetworks.length >= settings.max_network_cards) return;
		const defaultBridge = settings.bridges.length > 0 ? settings.bridges[0].name : '';
		vmNetworks = [
			...vmNetworks,
			{
				bridge: defaultBridge,
				model: 'virtio',
				mac: '',
				vlan: 0,
				rate_limit: '',
				mtu: 0,
				enabled: true
			}
		];
	}

	function removeNetworkCard(index: number): void {
		if (vmNetworks.length <= 1) return;
		vmNetworks = vmNetworks.filter((_, i) => i !== index);
	}

	function toggleTag(tag: string): void {
		if (vmTags.includes(tag)) {
			vmTags = vmTags.filter((t) => t !== tag);
		} else {
			vmTags = [...vmTags, tag];
		}
	}

	async function handleCreate(): Promise<void> {
		if (!settings || creating) return;
		creating = true;
		try {
			const request: VMCreateRequest = {
				name: vmName,
				node: vmNode,
				storage: vmStorage,
				description: vmDescription,
				iso: vmISO,
				tags: vmTags,
				sockets: vmSockets,
				cores: vmCores,
				memory_mb: vmMemoryGB * 1024,
				disks: vmDisks,
				networks: vmNetworks,
				enable_efi: vmEnableEFI,
				enable_tpm: vmEnableTPM,
				disk_bus: vmDiskBus,
				start_vm: vmStartAfterCreation
			};
			if (vmCloudInitEnabled) {
				const ciConfig: VMCreateCloudInit = {
					user: vmCIUser,
					password: vmCIPassword,
					ssh_keys: vmCISSHKeys,
					ip_config: vmCIIPConfig,
					ip: vmCIIP,
					gateway: vmCIGateway,
					dns: vmCIDNS,
					template_id: vmCITemplateID
				};
				request.cloud_init = ciConfig;
			}
			const resp = await createVM(request);
			if (resp.cloud_init_warning) {
				toast.warning(
					$t('vmCreate.toast.createdWithWarning', {
						values: { name: resp.name, warning: resp.cloud_init_warning }
					})
				);
			} else {
				toast.success(
					$t('vmCreate.toast.created', {
						values: { name: resp.name, vmid: String(resp.vmid) }
					})
				);
			}
			await goto(`/vm/${resp.vmid}`);
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			toast.error($t('vmCreate.toast.failed', { values: { error: msg } }));
		} finally {
			creating = false;
		}
	}

	function isStepValid(step: Step): boolean {
		if (!settings) return false;
		switch (step) {
			case 'base':
				return vmName.trim().length > 0 && vmNode !== '' && vmStorage !== '';
			case 'hardware':
				return (
					vmSockets >= settings.limits.sockets.min &&
					vmSockets <= settings.limits.sockets.max &&
					vmCores >= settings.limits.cores.min &&
					vmCores <= settings.limits.cores.max &&
					vmMemoryGB >= settings.limits.ram.min &&
					vmMemoryGB <= settings.limits.ram.max
				);
			case 'disk':
				return vmDisks.length > 0 && vmDisks.every((d) => d.size_gb > 0);
			case 'network':
				return vmNetworks.length > 0 && vmNetworks.every((n) => n.bridge !== '');
			case 'cloudinit':
				return true;
			case 'review':
				return true;
			default:
				return true;
		}
	}
</script>

<div class="mx-auto max-w-4xl space-y-6 p-4 md:p-6">
	<!-- Header -->
	<div>
		<h1 class="text-2xl font-bold tracking-tight">{$t('vmCreate.title')}</h1>
		<p class="text-muted-foreground text-sm">{$t('vmCreate.subtitle')}</p>
	</div>

	{#if loading}
		<div class="flex items-center justify-center py-20">
			<SpinnerGap class="text-muted-foreground h-8 w-8 animate-spin" />
		</div>
	{:else if !settings}
		<Card>
			<CardContent class="py-10 text-center">
				<Warning class="text-destructive mx-auto mb-3 h-10 w-10" />
				<p class="text-muted-foreground">{$t('common.error')}</p>
			</CardContent>
		</Card>
	{:else if !settings.proxmox_connected}
		<Card>
			<CardContent class="py-10 text-center">
				<Warning class="mx-auto mb-3 h-10 w-10 text-amber-500" />
				<p class="text-muted-foreground">{$t('vmCreate.offlineWarning')}</p>
			</CardContent>
		</Card>
	{:else if quotaBlocked}
		<Card>
			<CardContent class="py-10 text-center">
				<Warning class="mx-auto mb-3 h-10 w-10 text-amber-500" />
				<p class="text-muted-foreground">
					{$t('vmCreate.quotaReached', {
						values: {
							current: String(settings.max_vm_per_user),
							max: String(settings.max_vm_per_user)
						}
					})}
				</p>
			</CardContent>
		</Card>
	{:else}
		<!-- Step indicator -->
		<nav class="flex items-center justify-between gap-1 overflow-x-auto pb-2">
			{#each STEPS as step, i}
				{@const Icon = STEP_ICONS[step]}
				{@const isCurrent = i === currentStep}
				{@const isCompleted = i < currentStep}
				{@const valid = isStepValid(step)}
				<button
					type="button"
					onclick={() => goToStep(i)}
					class="flex min-w-0 flex-1 flex-col items-center gap-1 rounded-lg px-2 py-2 text-xs transition-colors
						{isCurrent
						? 'bg-primary/10 text-primary font-semibold'
						: isCompleted && valid
							? 'text-primary/70 hover:bg-muted'
							: 'text-muted-foreground hover:bg-muted'}"
				>
					<span
						class="flex h-8 w-8 items-center justify-center rounded-full border-2
							{isCurrent
							? 'border-primary bg-primary text-primary-foreground'
							: isCompleted && valid
								? 'border-primary/50 bg-primary/10 text-primary'
								: 'border-muted-foreground/30'}"
					>
						{#if isCompleted && valid}
							<Check class="h-4 w-4" />
						{:else}
							<Icon class="h-4 w-4" />
						{/if}
					</span>
					<span class="truncate">{$t(`vmCreate.steps.${step}`)}</span>
				</button>
				{#if i < STEPS.length - 1}
					<div
						class="mt-[-1rem] h-px w-4 flex-shrink-0
							{i < currentStep ? 'bg-primary/50' : 'bg-muted-foreground/20'}"
					></div>
				{/if}
			{/each}
		</nav>

		<!-- Step content -->
		<Card>
			<CardContent class="space-y-5 p-6">
				{#if currentStepName === 'base'}
					<!-- STEP 1: BASE -->
					<h2 class="text-lg font-semibold">{$t('vmCreate.base.title')}</h2>
					<Separator />
					<div class="grid gap-4 sm:grid-cols-2">
						<div class="space-y-2 sm:col-span-2">
							<Label for="vm-name">{$t('vmCreate.base.name')}</Label>
							<Input
								id="vm-name"
								bind:value={vmName}
								placeholder={$t('vmCreate.base.namePlaceholder')}
							/>
							<p class="text-muted-foreground text-xs">{$t('vmCreate.base.nameHint')}</p>
						</div>
						<div class="space-y-2">
							<Label>{$t('vmCreate.base.node')}</Label>
							<Select.Root
								type="single"
								value={vmNode}
								onValueChange={(v) => (vmNode = v)}
							>
								<Select.Trigger class="w-full">
									{vmNode || $t('vmCreate.base.selectNode')}
								</Select.Trigger>
								<Select.Content>
									{#each settings.nodes as node}
										<Select.Item value={node.name} disabled={node.disabled}>
											{node.name}
											{#if node.disabled}
												<span class="text-muted-foreground text-xs"> ({node.reason})</span>
											{/if}
										</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						{#if settings.nodes.every((n) => n.disabled)}
							<div class="bg-amber-50 border-amber-200 border rounded-lg p-3 text-sm">
								<p class="text-amber-800 font-medium">{$t('vmCreate.base.allNodesDisabled')}</p>
								<p class="text-amber-700 text-xs mt-1">{$t('vmCreate.base.allNodesDisabledHint')}</p>
							</div>
						{/if}
						{#if settings.storages.length === 0}
							<div class="bg-amber-50 border-amber-200 border rounded-lg p-3 text-sm">
								<p class="text-amber-800 font-medium">{$t('vmCreate.base.noStorages')}</p>
								<p class="text-amber-700 text-xs mt-1">{$t('vmCreate.base.noStoragesHint')}</p>
							</div>
						{/if}
						<div class="space-y-2">
							<Select.Root
								type="single"
								value={vmStorage}
								onValueChange={(v) => (vmStorage = v)}
							>
								<Select.Trigger class="w-full">
									{vmStorage || $t('vmCreate.base.selectStorage')}
								</Select.Trigger>
								<Select.Content>
									{#each settings.storages as storage}
										<Select.Item value={storage.name}>
											{storage.name}
											{#if storage.node}
												<span class="text-muted-foreground text-xs"> ({storage.node})</span>
											{/if}
										</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						<div class="space-y-2 sm:col-span-2">
							<Label>{$t('vmCreate.base.iso')}</Label>
							<Select.Root
								type="single"
								value={vmISO}
								onValueChange={(v) => (vmISO = v)}
							>
								<Select.Trigger class="w-full">
									{#if vmISO}
										{settings.isos.find((i) => i.volid === vmISO)?.name || vmISO}
									{:else}
										{$t('vmCreate.base.noIso')}
									{/if}
								</Select.Trigger>
								<Select.Content>
									<Select.Item value="">{$t('vmCreate.base.noIso')}</Select.Item>
									{#each settings.isos as iso}
										<Select.Item value={iso.volid}>{iso.name}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						<div class="space-y-2 sm:col-span-2">
							<Label for="vm-desc">{$t('vmCreate.base.description')}</Label>
							<Textarea
								id="vm-desc"
								bind:value={vmDescription}
								placeholder={$t('vmCreate.base.descriptionPlaceholder')}
								rows={2}
							/>
						</div>
						{#if settings.tags.length > 0}
							<div class="space-y-2 sm:col-span-2">
								<Label>{$t('vmCreate.base.tags')}</Label>
								<div class="flex flex-wrap gap-2">
									{#each settings.tags as tag}
										<button
											type="button"
											onclick={() => toggleTag(tag)}
											class="cursor-pointer"
										>
											<Badge variant={vmTags.includes(tag) ? 'default' : 'outline'}>
												{tag}
											</Badge>
										</button>
									{/each}
								</div>
							</div>
						{/if}
					</div>
				{:else if currentStepName === 'hardware'}
					<!-- STEP 2: HARDWARE -->
					<h2 class="text-lg font-semibold">{$t('vmCreate.hardware.title')}</h2>
					<Separator />
					<div class="grid gap-4 sm:grid-cols-2">
						<div class="space-y-2">
							<Label for="vm-sockets">{$t('vmCreate.hardware.sockets')}</Label>
							<Input
								id="vm-sockets"
								type="number"
								bind:value={vmSockets}
								min={settings.limits.sockets.min}
								max={settings.limits.sockets.max}
							/>
							<p class="text-muted-foreground text-xs">
								{settings.limits.sockets.min} – {settings.limits.sockets.max}
							</p>
						</div>
						<div class="space-y-2">
							<Label for="vm-cores">{$t('vmCreate.hardware.cores')}</Label>
							<Input
								id="vm-cores"
								type="number"
								bind:value={vmCores}
								min={settings.limits.cores.min}
								max={settings.limits.cores.max}
							/>
							<p class="text-muted-foreground text-xs">
								{settings.limits.cores.min} – {settings.limits.cores.max}
							</p>
						</div>
						<div class="space-y-2">
							<Label for="vm-memory">{$t('vmCreate.hardware.memory')}</Label>
							<Input
								id="vm-memory"
								type="number"
								bind:value={vmMemoryGB}
								min={settings.limits.ram.min}
								max={settings.limits.ram.max}
							/>
							<p class="text-muted-foreground text-xs">
								{settings.limits.ram.min} – {settings.limits.ram.max} GB
							</p>
						</div>
						<div class="flex items-end pb-6">
							<Badge variant="secondary" class="text-sm">
								{$t('vmCreate.hardware.totalVCPUs', { values: { count: String(totalVCPUs) } })}
							</Badge>
						</div>
						<div class="space-y-2 sm:col-span-2">
							<Label>{$t('vmCreate.hardware.diskBus')}</Label>
							<Select.Root
								type="single"
								value={vmDiskBus}
								onValueChange={(v) => (vmDiskBus = v)}
							>
								<Select.Trigger class="w-full">
									{DISK_BUSES.find((b) => b.value === vmDiskBus)?.label || vmDiskBus}
								</Select.Trigger>
								<Select.Content>
									{#each DISK_BUSES as bus}
										<Select.Item value={bus.value}>{bus.label}</Select.Item>
									{/each}
								</Select.Content>
							</Select.Root>
						</div>
						<div class="flex items-center gap-3 sm:col-span-2">
							<Switch bind:checked={vmEnableEFI} />
							<div>
								<p class="text-sm font-medium">{$t('vmCreate.hardware.enableEFI')}</p>
								<p class="text-muted-foreground text-xs">
									{$t('vmCreate.hardware.efiHint')}
								</p>
							</div>
						</div>
						<div class="flex items-center gap-3 sm:col-span-2">
							<Switch bind:checked={vmEnableTPM} />
							<div>
								<p class="text-sm font-medium">{$t('vmCreate.hardware.enableTPM')}</p>
								<p class="text-muted-foreground text-xs">
									{$t('vmCreate.hardware.tpmHint')}
								</p>
							</div>
						</div>
					</div>
				{:else if currentStepName === 'disk'}
					<!-- STEP 3: DISK -->
					<h2 class="text-lg font-semibold">{$t('vmCreate.disk.title')}</h2>
					<Separator />
					<div class="space-y-4">
						{#each vmDisks as disk, i}
							<div class="bg-muted/50 flex items-center gap-3 rounded-lg p-3">
								<div class="flex-1 space-y-1">
									<Label>
										{i === 0
											? $t('vmCreate.disk.primaryDisk')
											: $t('vmCreate.disk.diskIndex', { values: { index: String(i + 1) } })}
									</Label>
									<div class="flex items-center gap-2">
										<Input
											type="number"
											bind:value={vmDisks[i].size_gb}
											min={settings.limits.disk.min}
											max={settings.limits.disk.max}
											class="w-32"
										/>
										<span class="text-muted-foreground text-sm">GB</span>
										<span class="text-muted-foreground text-xs">
											({settings.limits.disk.min} – {settings.limits.disk.max})
										</span>
									</div>
								</div>
								{#if i > 0}
									<Button
										variant="ghost"
										size="sm"
										onclick={() => removeDisk(i)}
										class="text-destructive"
									>
										<Trash class="h-4 w-4" />
									</Button>
								{/if}
							</div>
						{/each}
						{#if vmDisks.length < settings.max_disk_per_vm}
							<Button variant="outline" size="sm" onclick={addDisk}>
								<Plus class="mr-1 h-4 w-4" />
								{$t('vmCreate.disk.addDisk')}
							</Button>
						{:else}
							<p class="text-muted-foreground text-xs">
								{$t('vmCreate.disk.maxDisksReached', {
									values: { max: String(settings.max_disk_per_vm) }
								})}
							</p>
						{/if}
					</div>
				{:else if currentStepName === 'network'}
					<!-- STEP 4: NETWORK -->
					<h2 class="text-lg font-semibold">{$t('vmCreate.network.title')}</h2>
					<Separator />
					<div class="space-y-4">
						{#each vmNetworks as net, i}
							<div class="bg-muted/50 space-y-3 rounded-lg p-4">
								<div class="flex items-center justify-between">
									<h3 class="text-sm font-medium">
										{$t('vmCreate.network.card', { values: { index: String(i + 1) } })}
									</h3>
									{#if vmNetworks.length > 1}
										<Button
											variant="ghost"
											size="sm"
											onclick={() => removeNetworkCard(i)}
											class="text-destructive"
										>
											<Trash class="h-4 w-4" />
										</Button>
									{/if}
								</div>
								<div class="grid gap-3 sm:grid-cols-2">
									<div class="space-y-1">
										<Label>{$t('vmCreate.network.bridge')}</Label>
										<Select.Root
											type="single"
											value={vmNetworks[i].bridge}
											onValueChange={(v) => {
												const updated = [...vmNetworks];
												updated[i] = { ...updated[i], bridge: v };
												vmNetworks = updated;
											}}
										>
											<Select.Trigger class="w-full">
												{vmNetworks[i].bridge || $t('vmCreate.network.selectBridge')}
											</Select.Trigger>
											<Select.Content>
												{#each settings.bridges as bridge}
													<Select.Item value={bridge.name}>
														{bridge.name}
														{#if bridge.description}
															<span class="text-muted-foreground text-xs"> — {bridge.description}</span>
														{/if}
													</Select.Item>
												{/each}
											</Select.Content>
										</Select.Root>
									</div>
									<div class="space-y-1">
										<Label>{$t('vmCreate.network.model')}</Label>
										<Select.Root
											type="single"
											value={vmNetworks[i].model}
											onValueChange={(v) => {
												const updated = [...vmNetworks];
												updated[i] = { ...updated[i], model: v };
												vmNetworks = updated;
											}}
										>
											<Select.Trigger class="w-full">
												{NETWORK_MODELS.find((m) => m.value === vmNetworks[i].model)?.label ||
													vmNetworks[i].model}
											</Select.Trigger>
											<Select.Content>
												{#each NETWORK_MODELS as model}
													<Select.Item value={model.value}>{model.label}</Select.Item>
												{/each}
											</Select.Content>
										</Select.Root>
									</div>
									<div class="space-y-1">
										<Label>{$t('vmCreate.network.mac')}</Label>
										<Input
											value={vmNetworks[i].mac}
											oninput={(e) => {
												const updated = [...vmNetworks];
												updated[i] = {
													...updated[i],
													mac: (e.target as HTMLInputElement).value
												};
												vmNetworks = updated;
											}}
											placeholder={$t('vmCreate.network.macPlaceholder')}
										/>
									</div>
									<div class="space-y-1">
										<Label>{$t('vmCreate.network.vlan')}</Label>
										<Input
											type="number"
											value={vmNetworks[i].vlan || ''}
											oninput={(e) => {
												const updated = [...vmNetworks];
												updated[i] = {
													...updated[i],
													vlan: parseInt((e.target as HTMLInputElement).value) || 0
												};
												vmNetworks = updated;
											}}
											placeholder={$t('vmCreate.network.vlanPlaceholder')}
											min={0}
											max={4096}
										/>
									</div>
								</div>
								<div class="flex items-center gap-3">
									<Switch
										checked={vmNetworks[i].enabled}
										onCheckedChange={(v) => {
											const updated = [...vmNetworks];
											updated[i] = { ...updated[i], enabled: v };
											vmNetworks = updated;
										}}
									/>
									<span class="text-sm">{$t('vmCreate.network.enabled')}</span>
								</div>
							</div>
						{/each}
						{#if vmNetworks.length < settings.max_network_cards}
							<Button variant="outline" size="sm" onclick={addNetworkCard}>
								<Plus class="mr-1 h-4 w-4" />
								{$t('vmCreate.network.addCard')}
							</Button>
						{:else}
							<p class="text-muted-foreground text-xs">
								{$t('vmCreate.network.maxCardsReached', {
									values: { max: String(settings.max_network_cards) }
								})}
							</p>
						{/if}
					</div>
				{:else if currentStepName === 'cloudinit'}
					<!-- STEP 5: CLOUD-INIT -->
					<h2 class="text-lg font-semibold">{$t('vmCreate.cloudinit.title')}</h2>
					<Separator />
					<div class="flex items-center gap-3">
						<Switch bind:checked={vmCloudInitEnabled} />
						<span class="text-sm font-medium">{$t('vmCreate.cloudinit.enable')}</span>
					</div>
					{#if vmCloudInitEnabled}
						<div class="grid gap-4 sm:grid-cols-2">
							{#if settings.cloudinit_templates.length > 0}
								<div class="space-y-2 sm:col-span-2">
									<Label>{$t('vmCreate.cloudinit.template')}</Label>
									<Select.Root
										type="single"
										value={vmCITemplateID}
										onValueChange={(v) => (vmCITemplateID = v)}
									>
										<Select.Trigger class="w-full">
											{#if vmCITemplateID}
												{settings.cloudinit_templates.find((t) => t.id === vmCITemplateID)
													?.name || vmCITemplateID}
											{:else}
												{$t('vmCreate.cloudinit.noTemplate')}
											{/if}
										</Select.Trigger>
										<Select.Content>
											<Select.Item value=""
												>{$t('vmCreate.cloudinit.noTemplate')}</Select.Item
											>
											{#each settings.cloudinit_templates as tpl}
												<Select.Item value={tpl.id}>
													{tpl.name}
													{#if tpl.description}
														<span class="text-muted-foreground text-xs">
															— {tpl.description}</span
														>
													{/if}
												</Select.Item>
											{/each}
										</Select.Content>
									</Select.Root>
								</div>
							{/if}
							<div class="space-y-2">
								<Label for="ci-user">{$t('vmCreate.cloudinit.user')}</Label>
								<Input
									id="ci-user"
									bind:value={vmCIUser}
									placeholder={$t('vmCreate.cloudinit.userPlaceholder')}
								/>
							</div>
							<div class="space-y-2">
								<Label for="ci-password">{$t('vmCreate.cloudinit.password')}</Label>
								<Input
									id="ci-password"
									type="password"
									bind:value={vmCIPassword}
									placeholder={$t('vmCreate.cloudinit.passwordPlaceholder')}
								/>
							</div>
							<div class="space-y-2 sm:col-span-2">
								<Label for="ci-ssh">{$t('vmCreate.cloudinit.sshKeys')}</Label>
								<Textarea
									id="ci-ssh"
									bind:value={vmCISSHKeys}
									placeholder={$t('vmCreate.cloudinit.sshKeysPlaceholder')}
									rows={3}
								/>
							</div>
							<div class="space-y-2 sm:col-span-2">
								<Label>{$t('vmCreate.cloudinit.ipConfig')}</Label>
								<div class="flex gap-3">
									<button
										type="button"
										onclick={() => (vmCIIPConfig = 'dhcp')}
										class="flex-1 rounded-lg border-2 p-3 text-center text-sm transition-colors
											{vmCIIPConfig === 'dhcp'
											? 'border-primary bg-primary/5'
											: 'border-muted hover:border-muted-foreground/30'}"
									>
										{$t('vmCreate.cloudinit.dhcp')}
									</button>
									<button
										type="button"
										onclick={() => (vmCIIPConfig = 'static')}
										class="flex-1 rounded-lg border-2 p-3 text-center text-sm transition-colors
											{vmCIIPConfig === 'static'
											? 'border-primary bg-primary/5'
											: 'border-muted hover:border-muted-foreground/30'}"
									>
										{$t('vmCreate.cloudinit.static')}
									</button>
								</div>
							</div>
							{#if vmCIIPConfig === 'static'}
								<div class="space-y-2">
									<Label for="ci-ip">{$t('vmCreate.cloudinit.ip')}</Label>
									<Input
										id="ci-ip"
										bind:value={vmCIIP}
										placeholder={$t('vmCreate.cloudinit.ipPlaceholder')}
									/>
								</div>
								<div class="space-y-2">
									<Label for="ci-gw">{$t('vmCreate.cloudinit.gateway')}</Label>
									<Input
										id="ci-gw"
										bind:value={vmCIGateway}
										placeholder={$t('vmCreate.cloudinit.gatewayPlaceholder')}
									/>
								</div>
							{/if}
							<div class="space-y-2">
								<Label for="ci-dns">{$t('vmCreate.cloudinit.dns')}</Label>
								<Input
									id="ci-dns"
									bind:value={vmCIDNS}
									placeholder={$t('vmCreate.cloudinit.dnsPlaceholder')}
								/>
							</div>
						</div>
					{/if}
				{:else if currentStepName === 'review'}
					<!-- STEP 6: REVIEW -->
					<h2 class="text-lg font-semibold">{$t('vmCreate.review.title')}</h2>
					<p class="text-muted-foreground text-sm">{$t('vmCreate.review.subtitle')}</p>

					<!-- Validation warnings -->
					{#if !isStepValid('base') || !isStepValid('hardware') || !isStepValid('disk') || !isStepValid('network')}
						<div class="border-destructive/50 bg-destructive/5 rounded-lg border p-4">
							<div class="flex items-center gap-2">
								<Warning class="text-destructive h-5 w-5 flex-shrink-0" />
								<p class="text-destructive text-sm font-medium">{$t('vmCreate.review.validationTitle')}</p>
							</div>
							<ul class="text-destructive/80 mt-2 space-y-1 pl-7 text-sm">
								{#if vmName.trim().length === 0}
									<li>{$t('vmCreate.review.missingName')}</li>
								{/if}
								{#if vmNode === ''}
									<li>{$t('vmCreate.review.missingNode')}</li>
								{/if}
								{#if vmStorage === ''}
									<li>{$t('vmCreate.review.missingStorage')}</li>
								{/if}
								{#if !isStepValid('hardware')}
									<li>{$t('vmCreate.review.invalidHardware')}</li>
								{/if}
								{#if !isStepValid('disk')}
									<li>{$t('vmCreate.review.invalidDisk')}</li>
								{/if}
								{#if !isStepValid('network')}
									<li>{$t('vmCreate.review.invalidNetwork')}</li>
								{/if}
							</ul>
						</div>
					{/if}

					<Separator />

					<!-- Base -->
					<div class="space-y-2">
						<h3 class="flex items-center gap-2 text-sm font-semibold">
							<Monitor class="h-4 w-4" />
							{$t('vmCreate.review.base')}
						</h3>
						<div class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm">
							<span class="text-muted-foreground">{$t('vmCreate.review.name')}</span>
							<span class="font-medium {vmName.trim().length === 0 ? 'text-destructive' : ''}">
								{vmName.trim().length > 0 ? vmName : $t('vmCreate.review.required')}
							</span>
							<span class="text-muted-foreground">{$t('vmCreate.review.node')}</span>
							<span class="{vmNode === '' ? 'text-destructive' : ''}">
								{vmNode || $t('vmCreate.review.required')}
							</span>
							<span class="text-muted-foreground">{$t('vmCreate.review.storage')}</span>
							<span class="{vmStorage === '' ? 'text-destructive' : ''}">
								{vmStorage || $t('vmCreate.review.required')}
							</span>
							<span class="text-muted-foreground">{$t('vmCreate.review.iso')}</span>
							<span>
								{#if vmISO}
									{settings.isos.find((i) => i.volid === vmISO)?.name || vmISO}
								{:else}
									{$t('vmCreate.review.noIso')}
								{/if}
							</span>
						</div>
					</div>

					<!-- Hardware -->
					<div class="space-y-2">
						<h3 class="flex items-center gap-2 text-sm font-semibold">
							<Cpu class="h-4 w-4" />
							{$t('vmCreate.review.hardware')}
						</h3>
						<div class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm">
							<span class="text-muted-foreground">{$t('vmCreate.review.sockets')}</span>
							<span>{vmSockets}</span>
							<span class="text-muted-foreground">{$t('vmCreate.review.cores')}</span>
							<span>{vmCores} ({totalVCPUs} vCPUs)</span>
							<span class="text-muted-foreground">{$t('vmCreate.review.memory')}</span>
							<span>{vmMemoryGB} GB</span>
							<span class="text-muted-foreground">{$t('vmCreate.review.diskBus')}</span>
							<span>{DISK_BUSES.find((b) => b.value === vmDiskBus)?.label || vmDiskBus}</span>
							<span class="text-muted-foreground">{$t('vmCreate.review.efi')}</span>
							<span>{vmEnableEFI ? $t('common.yes') : $t('common.no')}</span>
							<span class="text-muted-foreground">{$t('vmCreate.review.tpm')}</span>
							<span>{vmEnableTPM ? $t('common.yes') : $t('common.no')}</span>
						</div>
					</div>

					<!-- Disks -->
					<div class="space-y-2">
						<h3 class="flex items-center gap-2 text-sm font-semibold">
							<HardDrive class="h-4 w-4" />
							{$t('vmCreate.review.disk')}
						</h3>
						<div class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm">
							{#each vmDisks as disk, i}
								<span class="text-muted-foreground">
									{i === 0
										? $t('vmCreate.disk.primaryDisk')
										: $t('vmCreate.disk.diskIndex', { values: { index: String(i + 1) } })}
								</span>
								<span>{disk.size_gb} GB</span>
							{/each}
							<span class="text-muted-foreground font-medium"
								>{$t('vmCreate.review.totalDisk')}</span
							>
							<span class="font-medium">{totalDiskGB} GB</span>
						</div>
					</div>

					<!-- Networks -->
					<div class="space-y-2">
						<h3 class="flex items-center gap-2 text-sm font-semibold">
							<WifiHigh class="h-4 w-4" />
							{$t('vmCreate.review.network')}
						</h3>
						{#each vmNetworks as net, i}
							<div
								class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm"
							>
								<span class="text-muted-foreground"
									>{$t('vmCreate.network.card', { values: { index: String(i + 1) } })}</span
								>
								<span></span>
								<span class="text-muted-foreground">{$t('vmCreate.review.bridge')}</span>
								<span class="{net.bridge === '' ? 'text-destructive' : ''}">
									{#if net.bridge}
										{net.bridge}
										{@const desc = settings.bridges.find((b) => b.name === net.bridge)?.description}
										{#if desc}
											<span class="text-muted-foreground text-xs"> — {desc}</span>
										{/if}
									{:else}
										{$t('vmCreate.review.required')}
									{/if}
								</span>
								<span class="text-muted-foreground">{$t('vmCreate.review.model')}</span>
								<span
									>{NETWORK_MODELS.find((m) => m.value === net.model)?.label || net.model}</span
								>
								{#if net.vlan}
									<span class="text-muted-foreground">VLAN</span>
									<span>{net.vlan}</span>
								{/if}
							</div>
						{/each}
					</div>

					<!-- Cloud-init -->
					{#if vmCloudInitEnabled}
						<div class="space-y-2">
							<h3 class="flex items-center gap-2 text-sm font-semibold">
								<Cloud class="h-4 w-4" />
								{$t('vmCreate.review.cloudinit')}
							</h3>
							<div
								class="bg-muted/50 grid grid-cols-2 gap-x-4 gap-y-1 rounded-lg p-3 text-sm"
							>
								{#if vmCIUser}
									<span class="text-muted-foreground">{$t('vmCreate.cloudinit.user')}</span>
									<span>{vmCIUser}</span>
								{/if}
								<span class="text-muted-foreground">{$t('vmCreate.cloudinit.ipConfig')}</span>
								<span class="uppercase">{vmCIIPConfig}</span>
								{#if vmCIIPConfig === 'static' && vmCIIP}
									<span class="text-muted-foreground">IP</span>
									<span>{vmCIIP}</span>
								{/if}
								{#if vmCITemplateID}
									<span class="text-muted-foreground"
										>{$t('vmCreate.cloudinit.template')}</span
									>
									<span>
										{settings.cloudinit_templates.find((t) => t.id === vmCITemplateID)?.name ||
											vmCITemplateID}
									</span>
								{/if}
							</div>
						</div>
					{/if}

					<Separator />

					<!-- Start after creation -->
					<div class="flex items-center gap-3">
						<Switch bind:checked={vmStartAfterCreation} />
						<span class="text-sm font-medium">{$t('vmCreate.review.startAfterCreation')}</span>
					</div>
				{/if}
			</CardContent>
		</Card>

		<!-- Navigation buttons -->
		<div class="flex items-center justify-between">
			<Button variant="outline" onclick={goPrev} disabled={isFirstStep}>
				<CaretLeft class="mr-1 h-4 w-4" />
				{$t('common.previous')}
			</Button>
			{#if isLastStep}
				<Button
					onclick={handleCreate}
					disabled={creating || !isStepValid('base') || !isStepValid('hardware')}
				>
					{#if creating}
						<SpinnerGap class="mr-2 h-4 w-4 animate-spin" />
						{$t('vmCreate.review.creating')}
					{:else}
						<CheckCircle class="mr-2 h-4 w-4" />
						{$t('vmCreate.review.create')}
					{/if}
				</Button>
			{:else}
				<Button onclick={goNext} disabled={!isStepValid(currentStepName)}>
					{$t('common.next')}
					<CaretRight class="ml-1 h-4 w-4" />
				</Button>
			{/if}
		</div>
	{/if}
</div>
