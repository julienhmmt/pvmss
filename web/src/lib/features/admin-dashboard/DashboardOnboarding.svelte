<script lang="ts">
	/**
	 * First-run setup checklist on the admin dashboard. The server derives
	 * every step from stored state and omits the block once the required
	 * steps are done. An admin can also mark a step done by hand - a bypass
	 * for steps the instance cannot satisfy or chooses to skip.
	 */
	import Card from '#lib/shared/ui/Card.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { nextOnboardingStep, onboardingHref, onboardingLabel, type OnboardingStep } from './onboarding';

	interface Props {
		steps: OnboardingStep[];
		/** Persists the given step keys as manually done, then refreshes. */
		onMarkDone: (keys: string[]) => Promise<void>;
	}

	let { steps, onMarkDone }: Props = $props();

	let required = $derived(steps.filter((step) => step.required));
	let doneCount = $derived(required.filter((step) => step.done).length);
	let next = $derived(nextOnboardingStep(steps));
	let undoneKeys = $derived(steps.filter((step) => !step.done).map((step) => step.key));
	let marking = $state(false);
	let markError = $state(false);

	async function markDone(keys: string[]): Promise<void> {
		if (marking || keys.length === 0) return;
		marking = true;
		markError = false;
		try {
			await onMarkDone(keys);
		} catch {
			markError = true;
		} finally {
			marking = false;
		}
	}
</script>

<Card pad="none" title={m['admin.dashboard.onboarding.title']()} titleId="dashboard-onboarding-heading">
	{#snippet actions()}
		<Button variant="ghost" size="sm" loading={marking} onclick={() => void markDone(undoneKeys)} data-testid="onboarding-mark-all">
			{m['admin.dashboard.onboarding.markAllDone']()}
		</Button>
	{/snippet}
	<p class="px-5 pt-4 text-sm text-muted-foreground" data-testid="onboarding-progress">
		{m['admin.dashboard.onboarding.progress']({ done: doneCount, total: required.length })}
	</p>
	<ol class="divide-y divide-border pt-2" aria-labelledby="dashboard-onboarding-heading">
		{#each steps as step (step.key)}
			<li data-testid="onboarding-step" data-key={step.key} data-done={step.done} data-manual={step.manual || undefined}>
				{#if step.done}
					<span class="flex items-center gap-3 px-5 py-3 text-sm text-muted-foreground">
						<span class="w-4 shrink-0 text-success" aria-hidden="true">✓</span>
						<span class="min-w-0 flex-1 line-through">{onboardingLabel(step)}</span>
						<span class="sr-only"
							>({step.manual ? m['admin.dashboard.onboarding.markedDone']() : m['admin.dashboard.onboarding.done']()})</span
						>
					</span>
				{:else}
					<div class="flex items-center">
						<a
							href={onboardingHref(step)}
							class="flex min-w-0 flex-1 items-center gap-3 px-5 py-3 text-sm hover:bg-muted/60 pv-focus"
							aria-current={step === next ? 'step' : undefined}
						>
							<span class="h-2 w-2 shrink-0 rounded-full {step === next ? 'bg-primary' : 'bg-muted-foreground'}" aria-hidden="true"
							></span>
							<span class="min-w-0 flex-1 font-medium text-foreground">{onboardingLabel(step)}</span>
							{#if step === next}
								<span class="shrink-0 text-xs font-medium text-primary">{m['admin.dashboard.onboarding.next']()}</span>
							{:else if !step.required}
								<span class="shrink-0 text-xs text-muted-foreground">{m['admin.dashboard.onboarding.optional']()}</span>
							{/if}
							<span class="shrink-0 text-muted-foreground" aria-hidden="true">→</span>
						</a>
						<Button
							variant="link"
							class="mr-5"
							disabled={marking}
							onclick={() => void markDone([step.key])}
							data-testid="onboarding-mark-done"
						>
							{m['admin.dashboard.onboarding.markDone']()}
						</Button>
					</div>
				{/if}
			</li>
		{/each}
	</ol>
	{#if markError}
		<p class="px-5 py-3 text-xs text-destructive" role="alert">{m['admin.dashboard.onboarding.markError']()}</p>
	{/if}
</Card>
