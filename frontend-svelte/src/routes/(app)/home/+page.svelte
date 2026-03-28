<script lang="ts">
	import { t } from 'svelte-i18n';
	import { auth } from '$lib/stores/auth.svelte';
	import { Button } from '$lib/components/ui/button';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import { HouseIcon, PlusSquareIcon } from 'phosphor-svelte';
</script>

<div class="mx-auto max-w-5xl px-4 py-6">
	<PageHeader
		title={$t('user.home.title')}
		description={$t('user.home.description', { values: { username: auth.username } })}
		icon={HouseIcon}
	>
		{#snippet actions()}
			<Button href="/vm/create" size="sm">
				<PlusSquareIcon class="mr-2 h-4 w-4" />
				{$t('nav.createVm')}
			</Button>
		{/snippet}
	</PageHeader>

	<div class="rounded-lg border border-dashed border-border p-12 text-center text-muted-foreground">
		<HouseIcon class="mx-auto mb-3 h-10 w-10 opacity-30" />
		<p class="text-sm">{$t('user.home.migrationNote')}</p>
	</div>
</div>
