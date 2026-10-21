<script lang="ts">
	/**
	 * CommandPalette - keyboard-first jump to a page or a machine (ticket 06,
	 * v1 navigation only). Opened by Cmd/Ctrl+K or the header button; the
	 * shared Dialog owns the modal, Escape and focus return. Pages come from
	 * the sidebar's own destinations, machines from the VM list API (the
	 * same query as /search), so nothing the principal cannot open is offered.
	 */
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import { m } from '#lib/paraglide/messages.js';
	import Dialog from '#lib/shared/ui/Dialog.svelte';
	import { get } from '#lib/shared/api/client.js';
	import type { VmListResult } from '#lib/features/vms/list.svelte.js';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import { getChromeContext } from './chrome.svelte';
	import { filterItems, navigationItems, readRecent, rememberRecent, type PaletteItem } from './command-palette';

	const MACHINE_LIMIT = 8;
	const MACHINE_DEBOUNCE_MS = 150;

	const chrome = getChromeContext();
	const session = getSessionContext();

	let query = $state('');
	let activeIndex = $state(0);
	let machines = $state.raw<PaletteItem[]>([]);
	let machinesFailed = $state(false);
	// Re-read on every opening: another tab may have added a recent item.
	const recent = $derived(chrome.paletteOpen ? readRecent() : []);

	const GROUP_LABELS: Record<PaletteItem['group'], () => string> = {
		recent: () => m['palette.groupRecent'](),
		goto: () => m['palette.groupGoto'](),
		admin: () => m['palette.groupAdmin'](),
		machines: () => m['palette.groupMachines']()
	};

	const pages = $derived(filterItems(navigationItems(session.isAdmin), query));
	const results = $derived([...(query.trim() === '' ? recent : []), ...pages, ...machines]);
	const groups = $derived(
		(['recent', 'goto', 'admin', 'machines'] as const)
			.map((group) => ({ group, items: results.filter((item) => item.group === group) }))
			.filter((entry) => entry.items.length > 0)
	);
	const activeItem = $derived(results[activeIndex]);

	// Machines: debounced, latest query wins.
	let searchToken = 0;
	$effect(() => {
		const q = query.trim();
		const token = ++searchToken;
		if (q === '' || !chrome.paletteOpen) {
			machines = [];
			machinesFailed = false;
			return;
		}
		const timer = setTimeout(() => void searchMachines(q, token), MACHINE_DEBOUNCE_MS);
		return () => clearTimeout(timer);
	});

	async function searchMachines(q: string, token: number): Promise<void> {
		const params = new URLSearchParams({ search: q, scope: 'all', sortBy: 'name', sortDir: 'asc', pageSize: String(MACHINE_LIMIT) });
		try {
			const result = await get<VmListResult>(`/api/v1/vms?${params.toString()}`);
			if (token !== searchToken) return;
			machinesFailed = false;
			machines = result.items.map((vm) => {
				const href = resolve('/vms/[cluster]/[vmid]', { cluster: encodeURIComponent(vm.cluster), vmid: String(vm.vmid) });
				return { id: href, group: 'machines', label: vm.name || `VM ${vm.vmid}`, hint: `${vm.vmid} · ${vm.node} · ${vm.clusterDisplayName}`, href };
			});
		} catch {
			if (token !== searchToken) return;
			machines = [];
			machinesFailed = true;
		}
	}

	function close(): void {
		query = '';
		activeIndex = 0;
		chrome.closePalette();
	}

	function choose(item: PaletteItem | undefined): void {
		if (!item) return;
		rememberRecent(item);
		close();
		void goto(item.href);
	}

	function onKeydown(event: KeyboardEvent): void {
		const last = results.length - 1;
		const moves: Record<string, () => number> = {
			ArrowDown: () => (activeIndex >= last ? 0 : activeIndex + 1),
			ArrowUp: () => (activeIndex <= 0 ? last : activeIndex - 1),
			Home: () => 0,
			End: () => last
		};
		if (event.key === 'Enter') {
			event.preventDefault();
			choose(activeItem);
		} else if (moves[event.key] && results.length > 0) {
			event.preventDefault();
			activeIndex = moves[event.key]!();
			document.getElementById(`palette-option-${activeIndex}`)?.scrollIntoView({ block: 'nearest' });
		}
	}
</script>

<Dialog open={chrome.paletteOpen} labelledBy="palette-title" onClose={close} size="lg">
	<h2 id="palette-title" class="sr-only">{m['palette.title']()}</h2>
	<input
		type="text"
		bind:value={query}
		oninput={() => (activeIndex = 0)}
		onkeydown={onKeydown}
		role="combobox"
		aria-expanded="true"
		aria-controls="palette-list"
		aria-autocomplete="list"
		aria-activedescendant={activeItem ? `palette-option-${activeIndex}` : undefined}
		aria-label={m['palette.placeholder']()}
		placeholder={m['palette.placeholder']()}
		autocomplete="off"
		spellcheck="false"
		class="h-11 w-full rounded-[var(--radius-control)] border border-input bg-background px-3 text-sm pv-focus"
		data-testid="palette-input"
	/>
	<p class="sr-only" role="status" aria-live="polite">{m['palette.results']({ count: results.length })}</p>

	<div id="palette-list" role="listbox" aria-label={m['palette.title']()} class="mt-3 grid max-h-[50svh] gap-3 overflow-y-auto">
		{#each groups as { group, items } (group)}
			<div role="group" aria-labelledby="palette-group-{group}">
				<p id="palette-group-{group}" class="px-2 pb-1 text-2xs font-semibold uppercase tracking-[0.04em] text-muted-foreground">{GROUP_LABELS[group]()}</p>
				{#each items as item (item.group + item.id)}
					{@const index = results.indexOf(item)}
					<div
						id="palette-option-{index}"
						role="option"
						aria-selected={index === activeIndex}
						tabindex="-1"
						class="flex cursor-pointer items-baseline justify-between gap-3 rounded-[var(--radius-control)] px-2 py-2 text-sm {index === activeIndex ? 'bg-accent text-accent-foreground' : ''}"
						onclick={() => choose(item)}
						onkeydown={(event) => event.key === 'Enter' && choose(item)}
						onmousemove={() => (activeIndex = index)}
						data-testid="palette-option"
					>
						<span class="min-w-0 truncate">{item.label}</span>
						{#if item.hint}<span class="min-w-0 truncate text-xs text-muted-foreground" title={item.hint}>{item.hint}</span>{/if}
					</div>
				{/each}
			</div>
		{:else}
			<p class="px-2 py-2 text-sm text-muted-foreground">{m['palette.empty']()}</p>
		{/each}
		{#if machinesFailed}
			<p class="px-2 text-xs text-muted-foreground">{m['palette.machinesError']()}</p>
		{/if}
	</div>
</Dialog>
