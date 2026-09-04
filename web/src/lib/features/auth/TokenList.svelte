<script lang="ts">
	import { onMount } from 'svelte';
	import { setTokensContext, type TokenScope } from './tokens.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Button from '$lib/shared/ui/Button.svelte';
	import CopyButton from '$lib/shared/ui/CopyButton.svelte';
	import TableSkeleton from '$lib/shared/ui/TableSkeleton.svelte';
	import FormField from '$lib/shared/ui/FormField.svelte';
	import TextField from '$lib/shared/ui/TextField.svelte';
	import Select from '$lib/shared/ui/Select.svelte';
	import { getToastContext } from '$lib/shared/ui/toast.svelte';

	const store = setTokensContext();
	const toast = getToastContext();

	let label = $state('');
	let scope = $state<TokenScope>('read');

	onMount(() => {
		void store.load();
	});

	function createToken(event: SubmitEvent): void {
		event.preventDefault();
		const trimmed = label.trim();
		if (!trimmed) return;
		void store.create(trimmed, scope).then((ok) => {
			if (ok) toast.success(m['profile.tokens.createSuccess']());
			label = '';
		});
	}
</script>

<section class="w-full max-w-2xl">
	<h2 class="text-lg font-semibold tracking-tight">{m['profile.tokens.heading']()}</h2>

	<form class="mt-4 flex flex-wrap items-end gap-3" onsubmit={createToken}>
		<FormField
			label={m['profile.tokens.label']()}
			required
			error={store.error}
			class="min-w-0 flex-1"
		>
			{#snippet children({ id, describedBy, invalid })}
				<TextField {id} {describedBy} {invalid} bind:value={label} required />
			{/snippet}
		</FormField>
		<FormField label={m['profile.tokens.scope']()} class="w-44">
			{#snippet children({ id, describedBy, invalid })}
				<Select
					{id}
					{describedBy}
					{invalid}
					value={scope}
					onchange={(e: Event & { currentTarget: HTMLSelectElement }) => (scope = e.currentTarget.value as TokenScope)}
					options={[
						{ value: 'read', label: m['profile.tokens.scopeRead']() },
						{ value: 'read_write', label: m['profile.tokens.scopeReadWrite']() }
					]}
				/>
			{/snippet}
		</FormField>
		<Button type="submit" loading={store.creating}>{m['profile.tokens.createButton']()}</Button>
	</form>

	{#if store.lastCreatedValue}
		<div role="status" class="mt-3 rounded-md border border-border bg-card p-3 text-sm" aria-live="polite">
			<p class="font-medium">{m['profile.tokens.copyNow']()}</p>
			<div class="mt-1 flex flex-wrap items-center gap-2">
				<code class="block break-all rounded bg-muted px-2 py-1">{store.lastCreatedValue}</code>
				<CopyButton value={store.lastCreatedValue} />
			</div>
			<div class="mt-2">
				<Button variant="ghost" size="sm" onclick={() => store.dismissCreatedValue()}>
					{m['common.dismiss']()}
				</Button>
			</div>
		</div>
	{/if}

	{#if store.loading}
		<div role="status" aria-live="polite" class="sr-only">{m['common.loading']()}</div>
		<div class="mt-4">
			<TableSkeleton columns={5} />
		</div>
	{:else if store.tokens.length === 0}
		<EmptyState title={m['profile.tokens.empty']()} class="mt-6" />
	{:else}
		<table class="pv-table mt-4">
			<caption class="sr-only">{m['profile.tokens.caption']()}</caption>
			<thead>
				<tr class="border-b border-border">
					<th scope="col" class="font-medium">{m['profile.tokens.columnLabel']()}</th>
					<th scope="col" class="font-medium">{m['profile.tokens.columnScope']()}</th>
					<th scope="col" class="font-medium">{m['profile.tokens.columnCreated']()}</th>
					<th scope="col" class="font-medium">{m['profile.tokens.columnLastUsed']()}</th>
					<th scope="col" class="font-medium"><span class="sr-only">{m['common.actions']()}</span></th>
				</tr>
			</thead>
			<tbody>
				{#each store.tokens as token (token.id)}
					<tr class="border-b border-border last:border-0">
						<td class="font-medium">{token.label}</td>
						<td class="text-muted-foreground">{token.scope}</td>
						<td class="text-muted-foreground">{new Date(token.createdAt).toLocaleString()}</td>
						<td class="text-muted-foreground">{token.lastUsedAt ? new Date(token.lastUsedAt).toLocaleString() : m['profile.tokens.never']()}</td>
						<td class="text-right">
							<Button
							variant="ghost"
							size="sm"
							loading={store.revoking[token.id] === true}
							onclick={() =>
								void store.revoke(token.id).then((ok) => {
									if (ok) toast.success(m['profile.tokens.revokeSuccess']());
								})}
							label={m['profile.tokens.revoke']()}
						>
							{m['profile.tokens.revoke']()}
						</Button>
						</td>
					</tr>
				{/each}
			</tbody>
		</table>
	{/if}
</section>
