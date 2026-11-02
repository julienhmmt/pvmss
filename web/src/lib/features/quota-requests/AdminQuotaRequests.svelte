<script lang="ts">
	/**
	 * The admin inbox of pending quota requests. Approve writes the cap (the
	 * admin may edit the number first); deny leaves the policy untouched. A
	 * decision that lost a race to another admin (409) reloads the list.
	 */
	import { onMount } from 'svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import ConfirmDialog from '#lib/shared/ui/ConfirmDialog.svelte';
	import EmptyState from '#lib/shared/ui/EmptyState.svelte';
	import { ApiRequestError } from '#lib/shared/api/client.js';
	import { m } from '#lib/paraglide/messages.js';
	import {
		approveQuotaRequest,
		denyQuotaRequest,
		fetchAdminQuotaRequests,
		type QuotaRequest
	} from './quota-requests';

	let requests = $state<QuotaRequest[]>([]);
	let loading = $state(true);
	let loadError = $state<string | null>(null);
	let actionError = $state<string | null>(null);
	let busyId = $state<number | null>(null);
	/** Admin-edited allowance per request id; blank keeps the requested number. */
	let overrides = $state<Record<number, string>>({});
	/** A decision waiting for the admin's confirmation; null when no dialog is open. */
	let pendingDecision = $state<{ request: QuotaRequest; approve: boolean } | null>(null);

	async function load(): Promise<void> {
		loading = true;
		loadError = null;
		try {
			requests = await fetchAdminQuotaRequests('pending');
		} catch (failure: unknown) {
			loadError = failure instanceof ApiRequestError ? failure.message : m['admin.quotaRequests.loadError']();
		} finally {
			loading = false;
		}
	}

	onMount(() => void load());

	function quotaLabel(value: number): string {
		return value === -1 ? m['admin.quotaRequests.unlimited']() : String(value);
	}

	/** The number the admin typed, or undefined when the requested number is kept. */
	function typedAllowance(request: QuotaRequest): number | undefined {
		// bind:value on a number input yields a number (or null when cleared), not a string.
		const override = String(overrides[request.id] ?? '').trim();
		return override === '' ? undefined : Number(override);
	}

	/** Deny always asks; approve asks only when the typed allowance changes the request. */
	function requestDecision(request: QuotaRequest, approve: boolean): void {
		const granted = typedAllowance(request);
		if (!approve || (granted !== undefined && granted !== request.requestedQuota)) {
			pendingDecision = { request, approve };
			return;
		}
		void decide(request, approve);
	}

	function confirmPendingDecision(): void {
		const pending = pendingDecision;
		pendingDecision = null;
		if (pending) void decide(pending.request, pending.approve);
	}

	async function decide(request: QuotaRequest, approve: boolean): Promise<void> {
		busyId = request.id;
		actionError = null;
		try {
			if (approve) {
				await approveQuotaRequest(request.id, typedAllowance(request));
			} else {
				await denyQuotaRequest(request.id);
			}
			requests = requests.filter((item) => item.id !== request.id);
		} catch (failure: unknown) {
			actionError = failure instanceof ApiRequestError ? failure.message : m['admin.quotaRequests.actionError']();
			if (failure instanceof ApiRequestError && failure.status === 409) await load();
		} finally {
			busyId = null;
		}
	}
</script>

{#if loading}
	<p class="text-sm text-muted-foreground">{m['common.loading']()}</p>
{:else if loadError}
	<p role="alert" class="text-sm text-destructive">{loadError}</p>
{:else if requests.length === 0}
	<EmptyState title={m['admin.quotaRequests.emptyTitle']()} description={m['admin.quotaRequests.emptyBody']()} dataTestid="quota-requests-empty" />
{:else}
	{#if actionError}
		<p role="alert" class="mb-3 text-sm text-destructive">{actionError}</p>
	{/if}
	<div class="overflow-x-auto rounded-md border border-border bg-card">
		<table class="w-full text-left text-sm" data-testid="quota-requests-table">
			<thead class="bg-muted/50 text-xs uppercase tracking-wide text-muted-foreground">
				<tr>
					<th class="px-4 py-3 font-medium">{m['admin.quotaRequests.col.requester']()}</th>
					<th class="px-4 py-3 font-medium">{m['admin.quotaRequests.col.pool']()}</th>
					<th class="px-4 py-3 font-medium">{m['admin.quotaRequests.col.current']()}</th>
					<th class="px-4 py-3 font-medium">{m['admin.quotaRequests.col.requested']()}</th>
					<th class="px-4 py-3 font-medium">{m['admin.quotaRequests.col.reason']()}</th>
					<th class="px-4 py-3 font-medium">{m['admin.quotaRequests.col.actions']()}</th>
				</tr>
			</thead>
			<tbody class="divide-y divide-border">
				{#each requests as request (request.id)}
					<tr data-testid="quota-request-row">
						<td class="px-4 py-3 font-medium text-foreground">{request.requester}</td>
						<td class="px-4 py-3">{request.pool} <span class="text-muted-foreground">({request.cluster})</span></td>
						<td class="px-4 py-3">{quotaLabel(request.currentQuota)}</td>
						<td class="px-4 py-3">{quotaLabel(request.requestedQuota)}</td>
						<td class="max-w-xs break-words px-4 py-3 whitespace-pre-line">{request.reason}</td>
						<td class="px-4 py-3">
							<div class="flex min-w-[14rem] flex-col gap-2">
								<input
									type="number"
									class="h-9 rounded-md border border-input bg-background px-2 text-sm"
									aria-label={m['admin.quotaRequests.allowanceLabel']()}
									placeholder={String(request.requestedQuota)}
									bind:value={overrides[request.id]}
								/>
								<div class="flex gap-2">
									<Button disabled={busyId === request.id} onclick={() => requestDecision(request, true)}>
										{m['admin.quotaRequests.approve']()}
									</Button>
									<Button variant="secondary" disabled={busyId === request.id} onclick={() => requestDecision(request, false)}>
										{m['admin.quotaRequests.deny']()}
									</Button>
								</div>
							</div>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	</div>
{/if}

{#if pendingDecision}
	{@const pending = pendingDecision}
	{@const granted = typedAllowance(pending.request)}
	{#if pending.approve}
		<ConfirmDialog
			open
			title={m['admin.quotaRequests.approveConfirmTitle']()}
			message={m['admin.quotaRequests.approveConfirmBody']({
				requester: pending.request.requester,
				pool: pending.request.pool,
				requested: quotaLabel(pending.request.requestedQuota),
				granted: quotaLabel(granted ?? pending.request.requestedQuota)
			})}
			confirmLabel={m['admin.quotaRequests.approve']()}
			cancelLabel={m['common.cancel']()}
			testId="quota-decision-confirm"
			onConfirm={confirmPendingDecision}
			onClose={() => (pendingDecision = null)}
		/>
	{:else}
		<ConfirmDialog
			open
			title={m['admin.quotaRequests.denyConfirmTitle']({ requester: pending.request.requester })}
			message={m['admin.quotaRequests.denyConfirmBody']({ requester: pending.request.requester, pool: pending.request.pool })}
			confirmLabel={m['admin.quotaRequests.deny']()}
			cancelLabel={m['common.cancel']()}
			testId="quota-decision-confirm"
			onConfirm={confirmPendingDecision}
			onClose={() => (pendingDecision = null)}
		/>
	{/if}
{/if}
