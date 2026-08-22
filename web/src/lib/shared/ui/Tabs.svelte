<script lang="ts">
	import { m } from '$lib/paraglide/messages.js';

	interface Tab {
		id: string;
		label: () => string;
	}

	interface Props {
		tabs: Tab[];
		active: string;
	}

	let { tabs, active = $bindable() }: Props = $props();

	function selectTab(id: string): void {
		active = id;
	}

	function handleKeydown(event: KeyboardEvent, index: number): void {
		if (event.key !== 'ArrowRight' && event.key !== 'ArrowLeft') return;
		event.preventDefault();
		const delta = event.key === 'ArrowRight' ? 1 : -1;
		const nextIndex = (index + delta + tabs.length) % tabs.length;
		const next = tabs[nextIndex];
		if (!next) return;
		active = next.id;
		document.getElementById(`tab-${next.id}`)?.focus();
	}
</script>

<div
	role="tablist"
	class="flex flex-wrap gap-1 rounded-xl border border-border bg-muted/50 p-1"
	aria-label={m['common.tabsAriaLabel']()}
>
	{#each tabs as tab, index (tab.id)}
		<button
			type="button"
			id={`tab-${tab.id}`}
			role="tab"
			aria-selected={active === tab.id}
			aria-controls={`panel-${tab.id}`}
			tabindex={active === tab.id ? 0 : -1}
			class="rounded-lg px-3 py-2 text-sm font-medium transition-colors focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-ring focus-visible:ring-offset-2 focus-visible:ring-offset-background {active === tab.id
				? 'bg-card text-foreground shadow-card'
				: 'text-muted-foreground hover:bg-card/60 hover:text-foreground'}"
			onclick={() => selectTab(tab.id)}
			onkeydown={(event) => handleKeydown(event, index)}
			data-testid={`vm-tab-${tab.id}`}
		>
			{tab.label()}
		</button>
	{/each}
</div>
