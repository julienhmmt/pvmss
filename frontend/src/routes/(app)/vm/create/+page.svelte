<script lang="ts">
	import { onMount } from 'svelte';
	import { fly } from 'svelte/transition';
	import { goto } from '$app/navigation';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import { getVMCreateSettings, createVM } from '$lib/api/vm-create';
	import { tasks } from '$lib/stores/tasks.svelte';
	import { auth } from '$lib/stores/auth.svelte';
	import { createVMFormStore } from '$lib/stores/vm-create.svelte';
	import { Card, CardContent } from '$lib/components/ui/card';
	import { Button } from '$lib/components/ui/button';
	import { CaretLeft, CaretRight, CheckCircle, SpinnerGap, Warning } from 'phosphor-svelte';
	import WizardProgress from './_components/WizardProgress.svelte';
	import SimpleWizard from './_components/SimpleWizard.svelte';
	import StepBase from './_steps/StepBase.svelte';
	import StepHardware from './_steps/StepHardware.svelte';
	import StepDisk from './_steps/StepDisk.svelte';
	import StepNetwork from './_steps/StepNetwork.svelte';
	import StepCloudInit from './_steps/StepCloudInit.svelte';
	import StepReview from './_steps/StepReview.svelte';

	const store = createVMFormStore();

	let loading: boolean = $state(true);
	let creating: boolean = $state(false);
	let quotaBlocked: boolean = $state(false);
	let creationMode: 'simple' | 'advanced' = $state('simple');
	let currentStep: number = $state(0);
	// +1 when navigating forward, -1 backward — drives the fly transition direction
	let direction: number = $state(1);

	const settings = $derived(store.settings);
	const currentStepName = $derived(store.steps[currentStep] ?? 'base');
	const isFirstStep = $derived(currentStep === 0);
	const isLastStep = $derived(currentStep === store.steps.length - 1);
	const allStepsValid = $derived(store.steps.every((step) => store.isStepValid(step)));

	$effect(() => {
		const max = store.steps.length - 1;
		if (currentStep > max) currentStep = max;
	});

	// Draft persistence: every form change is saved to localStorage (best-effort)
	$effect(() => {
		store.saveDraft();
	});

	onMount(async () => {
		if (auth.isAdmin) {
			toast.error('Local admin users cannot create VMs');
			goto('/admin');
			return;
		}
		try {
			const loaded = await getVMCreateSettings();
			quotaBlocked = loaded.remainingVms !== -1 && loaded.remainingVms <= 0;
			if (store.init(loaded)) {
				toast.info($t('vmCreate.draftRestored'));
			}
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			toast.error(msg);
		} finally {
			loading = false;
		}
	});

	function goNext(): void {
		if (currentStep < store.steps.length - 1) {
			direction = 1;
			currentStep++;
		}
	}

	function goPrev(): void {
		if (currentStep > 0) {
			direction = -1;
			currentStep--;
		}
	}

	function goToStep(index: number): void {
		if (index >= 0 && index < store.steps.length) {
			direction = index > currentStep ? 1 : -1;
			currentStep = index;
		}
	}

	// resolveCloudInitWarning maps a stable backend warning code to a translated
	// message, falling back to the raw code when no translation is registered.
	function resolveCloudInitWarning(code: string): string {
		const key = `vmCreate.toast.warning.${code}`;
		const translated = $t(key);
		return translated === key ? code : translated;
	}

	async function handleCreate(): Promise<void> {
		if (!store.settings || creating) return;
		creating = true;
		try {
			const resp = await createVM(store.buildRequest());
			store.clearDraft();

			if (resp.upid && resp.node) {
				// Async creation: backend returned a Proxmox task UPID.
				// Track it in the tasks store; the Navbar will show progress.
				const taskId = tasks.add({
					kind: 'vmCreate',
					upid: resp.upid,
					node: resp.node,
					vmid: resp.vmid,
					label: resp.name,
				});

				toast.info($t('vmCreate.toast.creating', { values: { name: resp.name } }));

				// Capture translations eagerly before navigation — the component
				// may be destroyed during goto, but toast and $t are global singletons
				// that survive.  Eager capture avoids relying on reactive subscriptions
				// after the component is torn down.
				const createdMsg = $t('vmCreate.toast.created', { values: { name: resp.name, vmid: String(resp.vmid) } });

				// Register the completion callback BEFORE navigating away,
				// so it is in place if the task finishes during goto.
				tasks.onComplete(taskId, (task) => {
					if (task.status === 'stopped') {
						if (task.cloudInitWarning) {
							toast.warning(
								$t('vmCreate.toast.createdWithWarning', {
									values: { name: resp.name, warning: resolveCloudInitWarning(task.cloudInitWarning) }
								})
							);
						} else {
							toast.success(createdMsg);
						}
					} else {
						toast.error($t('vmCreate.toast.failed', { values: { error: task.exitStatus } }));
					}
					tasks.remove(taskId);
				});

				// Navigate to the VM detail page; it will load once Proxmox has it ready
				await goto(`/vm/${resp.vmid}`);
			} else {
				// Synchronous fallback (e.g. offline mode or future change)
				if (resp.cloudInitWarning) {
					toast.warning(
						$t('vmCreate.toast.createdWithWarning', {
							values: { name: resp.name, warning: resolveCloudInitWarning(resp.cloudInitWarning) }
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
			}
		} catch (err: unknown) {
			const msg = err instanceof Error ? err.message : 'Unknown error';
			toast.error($t('vmCreate.toast.failed', { values: { error: msg } }));
		} finally {
			creating = false;
		}
	}
</script>

<svelte:head>
	<title>PVMSS — {$t('vmCreate.title')}</title>
</svelte:head>

<div class="mx-auto space-y-6 px-4 py-6 pv-content-width">
	<!-- Header -->
	<div class="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
		<div>
			<h1 class="text-2xl font-bold tracking-tight">{$t('vmCreate.title')}</h1>
			<p class="text-muted-foreground text-sm">{$t('vmCreate.subtitle')}</p>
		</div>
		<!-- Mode toggle — only shown when creation is available -->
		{#if !loading && settings && settings.proxmoxConnected && !quotaBlocked}
			<div class="inline-flex self-start flex-shrink-0 rounded-lg border bg-muted p-1 gap-1">
				<button
					type="button"
					onclick={() => (creationMode = 'simple')}
					class="rounded-md px-4 py-1.5 text-sm font-medium transition-all
						{creationMode === 'simple'
						? 'bg-background text-foreground shadow-sm'
						: 'text-muted-foreground hover:text-foreground'}"
				>
					{$t('vmCreate.simple.modeSimple')}
				</button>
				<button
					type="button"
					onclick={() => { creationMode = 'advanced'; currentStep = 0; }}
					class="rounded-md px-4 py-1.5 text-sm font-medium transition-all
						{creationMode === 'advanced'
						? 'bg-background text-foreground shadow-sm'
						: 'text-muted-foreground hover:text-foreground'}"
				>
					{$t('vmCreate.simple.modeAdvanced')}
				</button>
			</div>
		{/if}
	</div>

	{#if loading}
		<LoadingSkeleton variant="form" rows={6} />
	{:else if !settings}
		<Card>
			<CardContent class="py-10 text-center">
				<Warning class="text-destructive mx-auto mb-3 h-10 w-10" />
				<p class="text-muted-foreground">{$t('common.error')}</p>
			</CardContent>
		</Card>
	{:else if !settings.proxmoxConnected}
		<Card>
			<CardContent class="py-10 text-center">
				<Warning class="mx-auto mb-3 h-10 w-10 text-warning" />
				<p class="text-muted-foreground">{$t('vmCreate.offlineWarning')}</p>
			</CardContent>
		</Card>
	{:else if quotaBlocked}
		<Card>
			<CardContent class="py-10 text-center">
				<Warning class="mx-auto mb-3 h-10 w-10 text-warning" />
				<p class="text-muted-foreground">
					{$t('vmCreate.quotaReached', {
						values: {
							current: String(settings.maxVmPerUser - settings.remainingVms),
							max: String(settings.maxVmPerUser)
						}
					})}
				</p>
			</CardContent>
		</Card>
	{:else if creationMode === 'simple'}
		<!-- ══════════════════════ SIMPLE MODE ══════════════════════ -->
		<SimpleWizard
			{store}
			{settings}
			{creating}
			onCreate={handleCreate}
			onSwitchToAdvanced={() => { creationMode = 'advanced'; currentStep = 0; }}
		/>
	{:else}
		<!-- ══════════════════════ ADVANCED MODE ══════════════════════ -->
		<WizardProgress
			steps={store.steps}
			current={currentStep}
			isStepValid={(step) => store.isStepValid(step)}
			onSelect={goToStep}
		/>

		{#key currentStep}
			<div in:fly={{ x: direction * 30, duration: 200 }}>
				<Card>
					<CardContent class="space-y-5 p-6">
						{#if currentStepName === 'base'}
							<StepBase {store} {settings} />
						{:else if currentStepName === 'hardware'}
							<StepHardware {store} {settings} />
						{:else if currentStepName === 'disk'}
							<StepDisk {store} {settings} />
						{:else if currentStepName === 'network'}
							<StepNetwork {store} {settings} />
						{:else if currentStepName === 'cloudinit'}
							<StepCloudInit {store} {settings} />
						{:else if currentStepName === 'review'}
							<StepReview {store} {settings} />
						{/if}
					</CardContent>
				</Card>
			</div>
		{/key}

		<!-- Navigation buttons -->
		<div class="flex items-center justify-between">
			<Button variant="outline" onclick={goPrev} disabled={isFirstStep}>
				<CaretLeft class="mr-1 h-4 w-4" />
				{$t('common.previous')}
			</Button>
			{#if isLastStep}
				<Button onclick={handleCreate} disabled={creating || !allStepsValid}>
					{#if creating}
						<SpinnerGap class="mr-2 h-4 w-4 animate-spin" />
						{$t('vmCreate.review.creating')}
					{:else}
						<CheckCircle class="mr-2 h-4 w-4" />
						{$t('vmCreate.review.create')}
					{/if}
				</Button>
			{:else}
				<Button onclick={goNext} disabled={!store.isStepValid(currentStepName)}>
					{$t('common.next')}
					<CaretRight class="ml-1 h-4 w-4" />
				</Button>
			{/if}
		</div>
	{/if}
</div>
