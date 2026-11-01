<script lang="ts">
	/**
	 * The "request an increase" affordance on the create page's quota-reached
	 * block. It shows the requester's latest request on their pool: a pending
	 * chip while an admin decides, the outcome once decided, or the button to
	 * file a new one. There is no push channel; the state is re-read on mount.
	 */
	import { onMount } from 'svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import Textarea from '#lib/shared/ui/Textarea.svelte';
	import { ApiRequestError } from '#lib/shared/api/client.js';
	import { m } from '#lib/paraglide/messages.js';
	import { fetchMyQuotaRequest, submitQuotaRequest, type QuotaRequest } from './quota-requests';

	interface Props {
		/** The pool's current allowance; the request must ask for more. */
		currentQuota: number;
	}

	let { currentQuota }: Props = $props();

	/** Mirrors the server's upper bound (policy.MaxVMPerUserLimit). */
	const MAX_QUOTA = 100000;
	const MAX_REASON = 500;

	let request = $state<QuotaRequest | null>(null);
	let loaded = $state(false);
	let open = $state(false);
	let requestedQuota = $state<string | number>('');
	let reason = $state('');
	let submitting = $state(false);
	let error = $state<string | null>(null);

	const quotaValid = $derived(
		Number.isInteger(Number(requestedQuota)) &&
			Number(requestedQuota) > currentQuota &&
			Number(requestedQuota) <= MAX_QUOTA
	);
	const trimmedReason = $derived(reason.trim());
	const canSubmit = $derived(quotaValid && trimmedReason !== '' && trimmedReason.length <= MAX_REASON && !submitting);

	async function refresh(): Promise<void> {
		try {
			request = await fetchMyQuotaRequest();
		} catch {
			// Unknown state: keep the request button. A duplicate is refused by
			// the server with the open request's id, which refresh picks up then.
			request = null;
		} finally {
			loaded = true;
		}
	}

	onMount(() => void refresh());

	async function submit(): Promise<void> {
		submitting = true;
		error = null;
		try {
			request = await submitQuotaRequest(Number(requestedQuota), trimmedReason);
			reason = '';
			requestedQuota = '';
			open = false;
		} catch (failure: unknown) {
			if (failure instanceof ApiRequestError && failure.code === 'quota_request_pending') {
				open = false;
				await refresh();
				return;
			}
			error = failure instanceof ApiRequestError ? failure.message : m['vms.create.quotaRequest.error']();
		} finally {
			submitting = false;
		}
	}
</script>

{#if loaded}
	<div class="flex flex-col items-center gap-3 px-5 pb-6 text-center" data-testid="quota-request-panel">
		{#if request?.status === 'pending'}
			<p class="rounded-full bg-muted px-3 py-1 text-sm text-muted-foreground" data-testid="quota-request-pending">
				{m['vms.create.quotaRequest.pending']({ quota: request.requestedQuota })}
			</p>
		{:else}
			{#if request?.status === 'approved'}
				<p class="text-sm text-muted-foreground" data-testid="quota-request-approved">
					{m['vms.create.quotaRequest.approved']({ quota: request.requestedQuota })}
				</p>
			{:else if request?.status === 'denied'}
				<p class="text-sm text-muted-foreground" data-testid="quota-request-denied">{m['vms.create.quotaRequest.denied']()}</p>
			{/if}
			<Button onclick={() => (open = true)}>{m['vms.create.quotaRequest.button']()}</Button>
		{/if}
	</div>
{/if}

<Dialog bind:open labelledBy="quota-request-title" onClose={() => (open = false)}>
	<h2 id="quota-request-title" class="mb-4 text-lg font-semibold">{m['vms.create.quotaRequest.title']()}</h2>
	<form
		onsubmit={(event) => {
			event.preventDefault();
			if (canSubmit) void submit();
		}}
	>
		<div class="space-y-4">
			<FormField label={m['vms.create.quotaRequest.quotaLabel']()} hint={m['vms.create.quotaRequest.quotaHint']({ current: currentQuota })} required error={error}>
				{#snippet children({ id, describedBy, invalid })}
					<TextField {id} {describedBy} {invalid} type="number" bind:value={requestedQuota} required data-testid="quota-request-quota" />
				{/snippet}
			</FormField>
			<FormField label={m['vms.create.quotaRequest.reasonLabel']()} hint={m['vms.create.quotaRequest.reasonHint']()} required>
				{#snippet children({ id, describedBy, invalid })}
					<Textarea {id} {describedBy} {invalid} bind:value={reason} rows={3} data-testid="quota-request-reason" />
				{/snippet}
			</FormField>
		</div>
		<div class="mt-6 flex justify-end gap-2">
			<Button variant="secondary" onclick={() => (open = false)}>{m['vms.create.quotaRequest.cancel']()}</Button>
			<Button type="submit" disabled={!canSubmit} loading={submitting}>{m['vms.create.quotaRequest.submit']()}</Button>
		</div>
	</form>
</Dialog>
