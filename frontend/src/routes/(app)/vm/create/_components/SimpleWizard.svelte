<script lang="ts">
	import { t } from 'svelte-i18n';
	import {
		CaretLeft,
		CaretRight,
		Check,
		CheckCircle,
		Cloud,
		Code,
		Cpu,
		Cube,
		Database,
		Flask,
		Globe,
		HardDrive,
		Info,
		MagicWand,
		Monitor,
		SpinnerGap
	} from 'phosphor-svelte';
	import { Card, CardContent } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import { Switch } from '$lib/components/ui/switch';
	import * as Select from '$lib/components/ui/select';
	import type { VMCreateSettings, VMProfileConfig } from '$lib/types/vm-create';
	import { profileColorClasses } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';

	const SIMPLE_STEPS = ['profile', 'details', 'confirm'] as const;

	// Maps icon name strings (from backend) to Phosphor icon components
	const PROFILE_ICONS: Record<string, typeof Globe> = {
		Globe,
		Code,
		Cube,
		Database,
		Flask,
		Monitor,
		Cpu,
		HardDrive,
		Cloud,
		Info
	};

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
		creating: boolean;
		onCreate: () => void;
		onSwitchToAdvanced: () => void;
	}

	let { store, settings, creating, onCreate, onSwitchToAdvanced }: Props = $props();

	let simpleStep: number = $state(0);

	const simpleDetailsValid = $derived(
		store.isNameValid && store.form.node !== '' && store.form.storage !== ''
	);

	function selectProfile(profile: VMProfileConfig): void {
		store.selectProfile(profile);
		// Move to details step if on confirm step
		if (simpleStep > 1) simpleStep = 1;
	}

	function simpleNext(): void {
		if (simpleStep < SIMPLE_STEPS.length - 1) simpleStep++;
	}

	function simplePrev(): void {
		if (simpleStep > 0) simpleStep--;
	}
</script>

<!-- Simple step indicator -->
<nav class="flex items-center gap-2">
	{#each SIMPLE_STEPS as step, i (i)}
		{@const isCurrent = i === simpleStep}
		{@const isCompleted = i < simpleStep}
		<div class="flex items-center gap-2">
			<div
				class="flex h-7 w-7 items-center justify-center rounded-full text-xs font-semibold transition-all
					{isCurrent
					? 'bg-primary text-primary-foreground'
					: isCompleted
						? 'bg-primary/20 text-primary'
						: 'bg-muted text-muted-foreground'}"
			>
				{#if isCompleted}
					<Check class="h-3.5 w-3.5" />
				{:else}
					{i + 1}
				{/if}
			</div>
			<span
				class="text-sm font-medium
					{isCurrent ? 'text-foreground' : 'text-muted-foreground'}"
			>
				{$t(`vmCreate.simple.step${step.charAt(0).toUpperCase() + step.slice(1)}`)}
			</span>
		</div>
		{#if i < SIMPLE_STEPS.length - 1}
			<div class="h-px flex-1 {i < simpleStep ? 'bg-primary/40' : 'bg-border'}"></div>
		{/if}
	{/each}
</nav>

{#if simpleStep === 0}
	<!-- ─── Profile selection ─── -->
	<div class="space-y-4">
		<div>
			<h2 class="text-lg font-semibold">{$t('vmCreate.simple.chooseProfile')}</h2>
			<p class="text-muted-foreground text-sm mt-1">{$t('vmCreate.simple.chooseProfileHint')}</p>
		</div>
		<div class="grid gap-3 sm:grid-cols-2">
			{#each settings.vmProfiles ?? [] as profile, i (i)}
				{@const ProfileIcon = PROFILE_ICONS[profile.icon] ?? Globe}
				{@const colors = profileColorClasses(profile.color)}
				{@const isSelected = store.selectedProfileId === profile.id}
				<button
					type="button"
					onclick={() => selectProfile(profile)}
					class="relative rounded-xl border-2 p-5 text-left transition-all
						{isSelected
						? 'border-primary bg-primary/5 shadow-sm'
						: 'border-border hover:border-muted-foreground/40 hover:bg-muted/30 hover:shadow-sm'}"
				>
					<div class="flex items-start gap-4">
						<div class="flex-shrink-0 rounded-lg p-2.5 {colors.bg}">
							<ProfileIcon class="h-5 w-5 {colors.icon}" />
						</div>
						<div class="min-w-0 flex-1">
							<p class="text-sm font-semibold leading-tight">{profile.name}</p>
							<p class="text-muted-foreground mt-1 text-xs leading-snug">{profile.description}</p>
							<div class="mt-3 flex flex-wrap gap-1.5">
								<span
									class="bg-muted inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium"
								>
									{$t('vmCreate.simple.specs.vcpus', {
										values: { count: String(profile.sockets * profile.cores) }
									})}
								</span>
								<span
									class="bg-muted inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium"
								>
									{$t('vmCreate.simple.specs.ram', {
										values: { gb: String(profile.ramGb) }
									})}
								</span>
								<span
									class="bg-muted inline-flex items-center rounded-md px-2 py-0.5 text-xs font-medium"
								>
									{$t('vmCreate.simple.specs.disk', {
										values: { gb: String(profile.diskGb) }
									})}
								</span>
							</div>
						</div>
						{#if isSelected}
							<div class="absolute right-3 top-3">
								<CheckCircle class="text-primary h-5 w-5" />
							</div>
						{/if}
					</div>
				</button>
			{/each}
		</div>
		<div class="flex justify-end">
			<Button onclick={simpleNext} disabled={!store.selectedProfileId}>
				{$t('common.next')}
				<CaretRight class="ml-1 h-4 w-4" />
			</Button>
		</div>
	</div>
{:else if simpleStep === 1}
	<!-- ─── VM Details ─── -->
	<Card>
		<CardContent class="space-y-5 p-6">
			<div>
				<h2 class="text-lg font-semibold">{$t('vmCreate.simple.details')}</h2>
				<p class="text-muted-foreground mt-1 text-sm">{$t('vmCreate.simple.detailsHint')}</p>
			</div>
			<Separator />
			<div class="grid gap-4 sm:grid-cols-2">
				<div class="space-y-2 sm:col-span-2">
					<Label for="simple-vm-name">{$t('vmCreate.base.name')}</Label>
					<Input
						id="simple-vm-name"
						bind:value={store.form.name}
						placeholder={$t('vmCreate.base.namePlaceholder')}
						autofocus
					/>
					<p
						class="text-xs {store.form.name.trim().length > 0 && !store.isNameValid
							? 'text-destructive'
							: 'text-muted-foreground'}"
					>
						{$t('vmCreate.base.nameHint')}
					</p>
				</div>

				<!-- Node selector — always shown, auto-selected hint when applicable -->
				<div class="space-y-1.5">
					<Label>{$t('vmCreate.base.node')}</Label>
					<Select.Root
						type="single"
						value={store.form.node}
						onValueChange={(v) => store.setNodeManually(v)}
					>
						<Select.Trigger class="w-full">
							{store.form.node || $t('vmCreate.base.selectNode')}
						</Select.Trigger>
						<Select.Content>
							{#each settings.nodes as node, i (i)}
								<Select.Item value={node.name} disabled={node.disabled}>
									{node.name}
									{#if node.disabled}
										<span class="text-muted-foreground text-xs"> ({node.reason})</span>
									{/if}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
					{#if store.nodeIsAuto}
						<p
							class="flex items-center gap-1 text-xs text-primary/70"
							title={$t('vmCreate.simple.autoNodeTooltip')}
						>
							<MagicWand class="h-3 w-3" />
							{$t('vmCreate.simple.autoSelected')}
						</p>
					{/if}
				</div>

				<!-- Storage selector — always shown, auto-selected hint when applicable -->
				<div class="space-y-1.5">
					<Label>{$t('vmCreate.base.storage')}</Label>
					<Select.Root
						type="single"
						value={store.form.storage}
						onValueChange={(v) => store.setStorageManually(v)}
					>
						<Select.Trigger class="w-full">
							{store.form.storage || $t('vmCreate.base.selectStorage')}
						</Select.Trigger>
						<Select.Content>
							{#each settings.storages as storage, i (i)}
								<Select.Item value={storage.name}>
									{storage.name}
									{#if storage.node === ''}
										<span class="text-muted-foreground text-xs"> — {$t('vmCreate.simple.sharedStorage')}</span>
									{:else if storage.node}
										<span class="text-muted-foreground text-xs"> ({storage.node})</span>
									{/if}
								</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
					{#if store.storageIsAuto}
						<p
							class="flex items-center gap-1 text-xs text-primary/70"
							title={store.autoStorageIsShared
								? $t('vmCreate.simple.autoStorageSharedTooltip')
								: $t('vmCreate.simple.autoStorageTooltip')}
						>
							<MagicWand class="h-3 w-3" />
							{store.autoStorageIsShared
								? $t('vmCreate.simple.autoSelectedShared')
								: $t('vmCreate.simple.autoSelected')}
						</p>
					{/if}
				</div>

				<div class="space-y-2 sm:col-span-2">
					<Label>{$t('vmCreate.base.iso')}</Label>
					<Select.Root
						type="single"
						value={store.form.iso}
						onValueChange={(v) => (store.form.iso = v ?? '')}
					>
						<Select.Trigger class="w-full">
							{#if store.form.iso}
								{settings.isos.find((i) => i.volid === store.form.iso)?.name || store.form.iso}
							{:else}
								{$t('vmCreate.base.noIso')}
							{/if}
						</Select.Trigger>
						<Select.Content>
							<Select.Item value="">{$t('vmCreate.base.noIso')}</Select.Item>
							{#each settings.isos as iso, i (i)}
								<Select.Item value={iso.volid}>{iso.name}</Select.Item>
							{/each}
						</Select.Content>
					</Select.Root>
				</div>
			</div>
		</CardContent>
	</Card>
	<div class="flex items-center justify-between">
		<Button variant="outline" onclick={simplePrev}>
			<CaretLeft class="mr-1 h-4 w-4" />
			{$t('common.previous')}
		</Button>
		<Button onclick={simpleNext} disabled={!simpleDetailsValid}>
			{$t('common.next')}
			<CaretRight class="ml-1 h-4 w-4" />
		</Button>
	</div>
{:else if simpleStep === 2}
	<!-- ─── Confirm & Create ─── -->
	<Card>
		<CardContent class="space-y-5 p-6">
			<div>
				<h2 class="text-lg font-semibold">{$t('vmCreate.simple.confirm')}</h2>
				<p class="text-muted-foreground mt-1 text-sm">{$t('vmCreate.simple.confirmHint')}</p>
			</div>
			<Separator />

			{#if store.selectedProfile}
				{@const ProfileIcon = PROFILE_ICONS[store.selectedProfile.icon] ?? Globe}
				{@const confirmColors = profileColorClasses(store.selectedProfile.color)}
				<!-- Profile + VM summary -->
				<div class="rounded-xl border-2 border-primary/20 bg-primary/5 p-4">
					<div class="mb-4 flex items-center gap-3">
						<div class="flex-shrink-0 rounded-lg p-2 {confirmColors.bg}">
							<ProfileIcon class="h-5 w-5 {confirmColors.icon}" />
						</div>
						<div>
							<p class="text-sm font-semibold">
								{store.selectedProfile.name}
							</p>
							<p class="text-muted-foreground text-xs">
								{store.selectedProfile.description}
							</p>
						</div>
					</div>
					<div class="grid grid-cols-2 gap-x-4 gap-y-1.5 text-sm">
						<span class="text-muted-foreground">{$t('vmCreate.review.name')}</span>
						<span class="font-medium">{store.form.name}</span>
						<span class="text-muted-foreground">{$t('vmCreate.review.node')}</span>
						<span>{store.form.node}</span>
						<span class="text-muted-foreground">{$t('vmCreate.review.storage')}</span>
						<span>{store.form.storage}</span>
						{#if store.form.iso}
							<span class="text-muted-foreground">{$t('vmCreate.review.iso')}</span>
							<span>{settings.isos.find((i) => i.volid === store.form.iso)?.name || store.form.iso}</span>
						{/if}
						<span class="text-muted-foreground">{$t('vmCreate.review.hardware')}</span>
						<span>
							{store.totalVCPUs} vCPUs · {store.form.memoryGB} GB RAM · {store.form.disks[0]?.sizeGb ?? 0} GB
						</span>
					</div>
				</div>
			{/if}

			<!-- Admin escalation notice -->
			<div
				class="flex gap-3 rounded-lg border border-warning-soft-border bg-warning-soft p-4"
			>
				<Info
					class="mt-0.5 h-5 w-5 flex-shrink-0 text-warning-soft-foreground"
				/>
				<div class="space-y-1">
					<p class="text-sm font-medium text-warning-soft-foreground">
						{$t('vmCreate.simple.adminNotice')}
					</p>
					<p class="text-xs text-warning-soft-foreground/80">
						{$t('vmCreate.simple.adminNoticeBody')}
					</p>
					<button
						type="button"
						onclick={onSwitchToAdvanced}
						class="text-xs font-medium text-warning-soft-foreground underline hover:opacity-80"
					>
						{$t('vmCreate.simple.switchToAdvanced')}
					</button>
				</div>
			</div>

			<!-- Start after creation -->
			<div class="flex items-center gap-3">
				<Switch bind:checked={store.form.startAfterCreation} />
				<span class="text-sm font-medium">{$t('vmCreate.review.startAfterCreation')}</span>
			</div>
		</CardContent>
	</Card>
	<div class="flex items-center justify-between">
		<Button variant="outline" onclick={simplePrev}>
			<CaretLeft class="mr-1 h-4 w-4" />
			{$t('common.previous')}
		</Button>
		<Button
			onclick={onCreate}
			disabled={creating || !simpleDetailsValid || !store.selectedProfileId}
		>
			{#if creating}
				<SpinnerGap class="mr-2 h-4 w-4 animate-spin" />
				{$t('vmCreate.review.creating')}
			{:else}
				<CheckCircle class="mr-2 h-4 w-4" />
				{$t('vmCreate.review.create')}
			{/if}
		</Button>
	</div>
{/if}
