<script lang="ts">
	import type { Snippet } from 'svelte';
	import { resolve } from '$app/paths';
	import { m } from '$lib/paraglide/messages.js';
	import TaskTray from '$lib/features/tasks/TaskTray.svelte';
	import { getSessionContext } from '$lib/features/auth/session.svelte';

	interface Props {
		/** Slot for the language switcher (US1) — empty until T016 wires it in. */
		languageSwitcher?: Snippet;
		/** Slot for the theme toggle (US2) — empty until T020 wires it in. */
		themeToggle?: Snippet;
		/** Slot for the status banner (US3) — rendered above the nav bar. */
		statusBanner?: Snippet;
	}

	let { languageSwitcher, themeToggle, statusBanner }: Props = $props();

	const session = getSessionContext();
</script>

{#if statusBanner}{@render statusBanner()}{/if}

<header class="border-b border-border">
	<nav class="mx-auto flex w-full max-w-5xl items-center justify-between px-4 py-3" aria-label="Main">
		<a href={resolve('/vms')} class="text-sm font-semibold tracking-tight">PVMSS</a>
		<div class="flex items-center gap-4">
			<a href={resolve('/vms')} class="text-sm text-muted-foreground hover:text-foreground">My VMs</a>
			<a href={resolve('/vms/create')} class="text-sm text-muted-foreground hover:text-foreground">Create</a>
			<a href={resolve('/nodes')} class="text-sm text-muted-foreground hover:text-foreground">Nodes</a>
			{#if session.isAdmin}
				<a href={resolve('/admin')} class="text-sm text-muted-foreground hover:text-foreground">Dashboard</a>
				<a href={resolve('/admin/nodes')} class="text-sm text-muted-foreground hover:text-foreground">Admin</a>
				<a href={resolve('/admin/clusters')} class="text-sm text-muted-foreground hover:text-foreground">Clusters</a>
				<a href={resolve('/admin/policy')} class="text-sm text-muted-foreground hover:text-foreground">Policy</a>
				<a href={resolve('/admin/policy/nodes')} class="text-sm text-muted-foreground hover:text-foreground">Node capacity</a>
				<a href={resolve('/admin/pools')} class="text-sm text-muted-foreground hover:text-foreground">Pools</a>
				<a href={resolve('/admin/settings')} class="text-sm text-muted-foreground hover:text-foreground">Settings</a>
				<a href={resolve('/admin/appinfo')} class="text-sm text-muted-foreground hover:text-foreground">App Info</a>
			{/if}
			<TaskTray />
			{#if languageSwitcher}{@render languageSwitcher()}{/if}
			{#if themeToggle}{@render themeToggle()}{/if}
		</div>
	</nav>
</header>
