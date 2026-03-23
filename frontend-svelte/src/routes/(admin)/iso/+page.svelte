<script lang="ts">
	import { onMount } from 'svelte';
	import { t } from 'svelte-i18n';
	import PageHeader from '$lib/components/layout/PageHeader.svelte';
	import LoadingSkeleton from '$lib/components/data/LoadingSkeleton.svelte';
	import LoadingToast from '$lib/components/data/LoadingToast.svelte';
	import ErrorBanner from '$lib/components/feedback/ErrorBanner.svelte';
	import EmptyState from '$lib/components/data/EmptyState.svelte';
	import { Switch } from '$lib/components/ui/switch';
	import { getISOs, toggleISO } from '$lib/api/admin/iso';
	import { formatBytes } from '$lib/utils/format';
	import { Disc } from 'phosphor-svelte';
	import { toast } from 'svelte-sonner';
	import type { ISO } from '$lib/types/admin';
	import * as Table from '$lib/components/ui/table';

	let loading = $state(true);
	let refreshing = $state(false);
	let error = $state<Error | null>(null);
	let isos = $state<ISO[]>([]);

	async function load() {
		if (isos.length > 0) {
			refreshing = true;
		} else {
			loading = true;
		}
		error = null;
		try {
			isos = await getISOs();
		} catch (e) {
			error = e as Error;
		} finally {
			loading = false;
			refreshing = false;
		}
	}

	async function handleToggle(volid: string) {
		try {
			await toggleISO(volid);
			await load();
		} catch (e) {
			toast.error((e as Error).message);
		}
	}

	onMount(load);
</script>

<PageHeader title={$t('admin.iso.title')} icon={Disc} />

<LoadingToast visible={refreshing} />

{#if error}
	<ErrorBanner {error} onRetry={load} />
{:else if loading}
	<LoadingSkeleton variant="table" rows={5} />
{:else if isos.length === 0}
	<EmptyState title={$t('admin.iso.noIso')} icon={Disc} />
{:else}
	<div class="rounded-md border">
		<Table.Root>
			<Table.Header>
				<Table.Row>
					<Table.Head>{$t('common.name')}</Table.Head>
					<Table.Head>{$t('common.storage')}</Table.Head>
					<Table.Head>{$t('common.size')}</Table.Head>
					<Table.Head>{$t('common.enabled')}</Table.Head>
				</Table.Row>
			</Table.Header>
			<Table.Body>
				{#each isos as iso}
					<Table.Row>
						<Table.Cell class="font-medium">{iso.name}</Table.Cell>
						<Table.Cell>{iso.storage}</Table.Cell>
						<Table.Cell>{formatBytes(iso.size)}</Table.Cell>
						<Table.Cell>
							<Switch
								checked={iso.enabled}
								onCheckedChange={() => handleToggle(iso.volid)}
							/>
						</Table.Cell>
					</Table.Row>
				{/each}
			</Table.Body>
		</Table.Root>
	</div>
{/if}
