<script lang="ts">
	import { page } from '$app/state';
	import { replaceState } from '$app/navigation';
	import { formatDateTime } from '#lib/shared/format-date.js';
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import Pill from '#lib/shared/ui/Pill.svelte';
	import Select from '#lib/shared/ui/Select.svelte';
	import Skeleton from '#lib/shared/ui/Skeleton.svelte';
	import { statusLabel, statusTone } from './clusters.svelte';
	import {
		fetchClusterTests,
		parseTestHistoryLimit,
		TEST_HISTORY_LIMITS,
		type ClusterTestEntry,
		type TestHistoryLimit
	} from './cluster-detail.js';
	import { m } from '#lib/paraglide/messages.js';

	interface Props {
		name: string;
	}

	let { name }: Props = $props();

	let entries = $state.raw<ClusterTestEntry[]>([]);
	let loading = $state.raw(true);
	let error = $state.raw(false);
	let limit: TestHistoryLimit = $state(parseTestHistoryLimit(page.url.searchParams.get('tests')));

	const limitOptions = TEST_HISTORY_LIMITS.map((value) => ({ value: String(value), label: String(value) }));

	$effect(() => {
		void load(name, limit);
	});

	async function load(target: string, size: TestHistoryLimit): Promise<void> {
		loading = true;
		error = false;
		try {
			entries = await fetchClusterTests(target, size);
		} catch {
			error = true;
		} finally {
			loading = false;
		}
	}

	function selectLimit(raw: string): void {
		limit = parseTestHistoryLimit(raw);
		const url = new URL(page.url.href);
		if (limit === 5) url.searchParams.delete('tests');
		else url.searchParams.set('tests', String(limit));
		replaceState(`${url.pathname}${url.search}`, page.state);
	}

	/** Shows a test the page just ran without refetching the list. */
	export function prepend(entry: ClusterTestEntry): void {
		entries = [entry, ...entries].slice(0, limit);
	}
</script>

<Card pad="none" title={m['admin.clusterDetail.testHistory']()} titleId="cluster-test-history-section">
	{#snippet actions()}
		<label class="flex items-center gap-2 text-xs text-muted-foreground">
			{m['admin.clusterDetail.testHistoryShow']()}
			<Select class="w-20" bind:value={() => String(limit), selectLimit} options={limitOptions} aria-label={m['admin.clusterDetail.testHistoryShow']()} />
		</label>
	{/snippet}
	{#if error}
		<div class="p-5"><Alert>{m['admin.clusterDetail.testHistoryError']()}</Alert></div>
	{:else if loading && entries.length === 0}
		<div class="p-5"><Skeleton class="h-24 w-full" /></div>
	{:else if entries.length === 0}
		<p class="px-5 py-4 text-sm text-muted-foreground">{m['admin.clusterDetail.testHistoryEmpty']()}</p>
	{:else}
		<div class="overflow-x-auto">
			<table class="pv-table pv-responsive-table" data-testid="cluster-test-history">
				<caption class="sr-only">{m['admin.clusterDetail.testHistory']()}</caption>
				<thead><tr>
					<th class="font-medium">{m['admin.clusterDetail.testHistoryWhen']()}</th><th class="font-medium">{m['common.status']()}</th><th class="font-medium">{m['admin.clusterDetail.testHistoryMessage']()}</th><th class="font-medium">{m['admin.clusters.version']()}</th><th class="font-medium">{m['admin.clusterDetail.testHistoryActor']()}</th><th class="text-right font-medium">{m['admin.clusterDetail.testHistoryDuration']()}</th>
				</tr></thead>
				<tbody>
					{#each entries as entry, index (`${entry.testedAt}-${index}`)}
						<tr>
							<td class="font-mono text-xs" data-label={m['admin.clusterDetail.testHistoryWhen']()}><time datetime={entry.testedAt}>{formatDateTime(entry.testedAt)}</time></td>
							<td data-label={m['common.status']()}><Pill tone={statusTone(entry.status)} label={statusLabel(entry.status)} /></td>
							<td class="text-muted-foreground" data-label={m['admin.clusterDetail.testHistoryMessage']()}>{entry.message ?? ' - '}</td>
							<td class="font-mono" data-label={m['admin.clusters.version']()}>{entry.proxmoxVersion ?? ' - '}</td>
							<td class="font-mono" data-label={m['admin.clusterDetail.testHistoryActor']()}>{entry.actor ?? ' - '}</td>
							<td class="text-right font-mono tabular-nums" data-label={m['admin.clusterDetail.testHistoryDuration']()}>{entry.durationMs === null ? ' - ' : `${entry.durationMs} ms`}</td>
						</tr>
					{/each}
				</tbody>
			</table>
		</div>
	{/if}
</Card>
