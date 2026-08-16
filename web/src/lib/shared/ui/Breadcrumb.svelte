<script lang="ts">
	interface Crumb {
		label: string;
		href?: string;
	}

	interface Props {
		items: Crumb[];
	}

	let { items }: Props = $props();
</script>

<nav aria-label="Breadcrumb" class="mb-4">
	<ol class="flex flex-wrap items-center gap-2 text-sm text-muted-foreground">
		{#each items as item, i (item.label)}
			<li class="flex items-center gap-2">
				{#if i > 0}<span aria-hidden="true" class="text-border">/</span>{/if}
				{#if item.href && i < items.length - 1}
					<a href={item.href} class="hover:text-foreground hover:underline">{item.label}</a>
				{:else}
					<span class={i === items.length - 1 ? 'font-medium text-foreground' : ''}>{item.label}</span>
				{/if}
			</li>
		{/each}
	</ol>
</nav>
