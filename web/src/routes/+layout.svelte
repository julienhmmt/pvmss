<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onDestroy, onMount } from 'svelte';
	import { resolve } from '$app/paths';
	import '../app.css';
	import { setTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import TaskTray from '$lib/features/tasks/TaskTray.svelte';
	import { setSessionContext } from '$lib/features/auth/session.svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	// The task tray is global (FR-015): one instance for the whole shell,
	// mounted in the navbar so task progress survives in-app navigation.
	const tray = setTaskTrayContext();
	onDestroy(() => tray.destroy());

	// The session context powers the admin nav link visibility (FR-008):
	// non-admins never see the link, so they don't click into a 403. The
	// server-side RequireAdmin middleware remains the real guard.
	const session = setSessionContext();
	onMount(() => session.load());
</script>

<div class="flex min-h-screen flex-col bg-background text-foreground">
	<header class="border-b border-border">
		<nav class="mx-auto flex w-full max-w-5xl items-center justify-between px-4 py-3" aria-label="Main">
			<a href={resolve('/vms')} class="text-sm font-semibold tracking-tight">PVMSS</a>
			<div class="flex items-center gap-4">
				<a href={resolve('/vms')} class="text-sm text-muted-foreground hover:text-foreground">My VMs</a>
				<a href={resolve('/vms/create')} class="text-sm text-muted-foreground hover:text-foreground">Create</a>
				<a href={resolve('/nodes')} class="text-sm text-muted-foreground hover:text-foreground">Nodes</a>
				{#if session.isAdmin}
					<a href={resolve('/admin/nodes')} class="text-sm text-muted-foreground hover:text-foreground">Admin</a>
					<a href={resolve('/admin/policy')} class="text-sm text-muted-foreground hover:text-foreground">Policy</a>
					<a href={resolve('/admin/policy/nodes')} class="text-sm text-muted-foreground hover:text-foreground">Node capacity</a>
					<a href={resolve('/admin/pools')} class="text-sm text-muted-foreground hover:text-foreground">Pools</a>
				{/if}
				<TaskTray />
			</div>
		</nav>
	</header>
	<main class="flex flex-1 flex-col items-center justify-center">
		{@render children()}
	</main>
</div>
