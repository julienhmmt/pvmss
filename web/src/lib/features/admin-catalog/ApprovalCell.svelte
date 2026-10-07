<script lang="ts">
	/**
	 * ApprovalCell - the catalog tables' "Status" column. An approved item
	 * gets a Switch plus a worded state ("Approved" / "Approve") so the
	 * column is never colour-only; a `missing` item (a stored approval the
	 * cluster no longer reports) cannot be toggled and instead offers the
	 * Remove action that forgets the stale row.
	 */
	import Switch from '$lib/shared/ui/Switch.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import { m } from '$lib/paraglide/messages.js';

	interface Props {
		enabled: boolean;
		/** Item name, used in the switch's accessible label. */
		name: string;
		/** In-flight toggle: shows a pending ellipsis, keeps aria-busy. */
		pending: boolean;
		/** Stored approval whose item Proxmox no longer reports. */
		missing?: boolean | undefined;
		/** Extra disable (e.g. a template whose disk read failed). */
		disabled?: boolean | undefined;
		onToggle: () => void;
		/** Renders the Remove button in the missing branch when set. */
		onRemove?: () => void;
		removeTestId?: string;
		labelTestId?: string;
	}

	let {
		enabled,
		name,
		pending,
		missing = false,
		disabled = false,
		onToggle,
		onRemove,
		removeTestId,
		labelTestId
	}: Props = $props();
</script>

{#if missing}
	<div class="flex items-center gap-2">
		<span class="text-xs text-muted-foreground">{m['admin.catalog.missingBadge']()}</span>
		{#if onRemove}
			<Button variant="ghost" size="sm" onclick={onRemove} data-testid={removeTestId}>
				{m['admin.catalog.remove']()}
			</Button>
		{/if}
	</div>
{:else}
	<span class="inline-flex items-center gap-2" aria-busy={pending}>
		<Switch
			checked={enabled}
			{disabled}
			label={enabled ? m['admin.catalog.revokeApproval']({ name }) : m['admin.catalog.approveName']({ name })}
			{onToggle}
		/>
		<span class="text-xs text-muted-foreground" data-testid={labelTestId}>
			{#if pending}
				…
			{:else}
				{enabled ? m['admin.catalog.approvedStatus']() : m['admin.catalog.approveAction']()}
			{/if}
		</span>
	</span>
{/if}
