<script lang="ts">
	import type { Snippet } from 'svelte';
	import { onDestroy } from 'svelte';
	import { resolve } from '$app/paths';
	import '../app.css';
	import { setTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import TaskTray from '$lib/features/tasks/TaskTray.svelte';

	interface Props {
		children: Snippet;
	}

	let { children }: Props = $props();

	// The task tray is global (FR-015): one instance for the whole shell,
	// mounted in the navbar so task progress survives in-app navigation.
	const tray = setTaskTrayContext();
	onDestroy(() => tray.destroy());
</script>

<div class="flex min-h-screen flex-col bg-background text-foreground">
	<header class="border-b border-border">
		<nav class="mx-auto flex w-full max-w-5xl items-center justify-between px-4 py-3" aria-label="Main">
			<a href={resolve('/vms')} class="text-sm font-semibold tracking-tight">PVMSS</a>
			<div class="flex items-center gap-4">
				<a href={resolve('/vms')} class="text-sm text-muted-foreground hover:text-foreground">My VMs</a>
				<a href={resolve('/vms/create')} class="text-sm text-muted-foreground hover:text-foreground">Create</a>
				<a href={resolve('/nodes')} class="text-sm text-muted-foreground hover:text-foreground">Nodes</a>
				<TaskTray />
			</div>
		</nav>
	</header>
	<main class="flex flex-1 flex-col items-center justify-center">
		{@render children()}
	</main>
</div>
