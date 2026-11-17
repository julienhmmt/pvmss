<script lang="ts">
	/**
	 * NotificationBell - header bell for pool users: an unread badge and a
	 * menu of recent notifications (newest first). Opening it marks
	 * everything seen.
	 */
	import { goto } from '$app/navigation';
	import Menu, { type MenuItem } from '#lib/shared/ui/Menu.svelte';
	import BellIcon from '#lib/shared/ui/icons/BellIcon.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import type { NotificationsState } from './notifications.svelte';

	let { state }: { state: NotificationsState } = $props();

	const items = $derived<MenuItem[]>(
		state.items.length === 0
			? [{ id: 'empty', label: m['notifications.empty'](), disabled: true, onclick: () => {} }]
			: state.items.map((item) => ({
					id: item.id,
					label: item.message,
					destructive: item.kind === 'error',
					onclick: () => {
						if (item.href) void goto(item.href);
					}
				}))
	);
	const label = $derived(
		state.unread > 0 ? m['notifications.labelUnread']({ count: state.unread }) : m['notifications.label']()
	);
</script>

<Menu {label} {items} size="sm" testId="notification-bell" onOpen={() => state.markSeen()}>
	{#snippet trigger()}
		<BellIcon class="h-4 w-4" />
		{#if state.unread > 0}
			<span
				class="absolute -top-1 -right-1 grid h-4 min-w-4 place-items-center rounded-full bg-primary-solid px-1 text-2xs font-semibold text-primary-foreground tabular-nums"
				aria-hidden="true"
				data-testid="notification-badge">{state.unread > 9 ? '9+' : state.unread}</span
			>
		{/if}
	{/snippet}
</Menu>
