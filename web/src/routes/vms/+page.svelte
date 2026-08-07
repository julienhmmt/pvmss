<script lang="ts">
	import { onMount, onDestroy } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { page } from '$app/state';
	import { setVmListContext } from '$lib/features/vms/list.svelte';
	import { getTaskTrayContext } from '$lib/features/tasks/tasks.svelte';
	import VmList from '$lib/features/vms/VmList.svelte';

	// Wiring only: the list state, URL sync, and rendering all live in
	// $lib/features/vms (FR-010) — this page just picks the scope.
	const vmListStore = setVmListContext({
		scope: 'mine',
		initialQuery: page.url.search,
		navigate: (queryString: string) => {
			void goto(`${resolve('/vms')}${queryString === '' ? '' : `?${queryString}`}`, {
				replaceState: true,
				noScroll: true,
				keepFocus: true
			});
		}
	});

	let offTaskOk: (() => void) | null = null;
	onMount(() => {
		void vmListStore.load();
		offTaskOk = getTaskTrayContext().onTaskOk(() => void vmListStore.load());
	});
	onDestroy(() => {
		offTaskOk?.();
	});
</script>

<svelte:head>
	<title>My VMs — PVMSS</title>
</svelte:head>

<section class="mx-auto w-full max-w-5xl px-4 py-8">
	<div class="mb-4 flex items-center justify-between">
		<h1 class="text-2xl font-semibold tracking-tight">My VMs</h1>
		<a
			href={resolve('/vms/create')}
			class="rounded-md bg-primary px-3 py-2 text-sm font-medium text-primary-foreground"
		>
			Create a VM
		</a>
	</div>

	{#if vmListStore.loading && vmListStore.result === null}
		<p role="status" aria-live="polite" class="text-muted-foreground">Loading…</p>
	{:else}
		<VmList />
	{/if}
</section>
