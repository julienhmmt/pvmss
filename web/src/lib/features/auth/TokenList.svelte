<script lang="ts">
	import { onMount } from 'svelte';
	import { setTokensContext, type TokenScope } from './tokens.svelte';
	import { m } from '$lib/paraglide/messages.js';
	import EmptyState from '$lib/shared/ui/EmptyState.svelte';
	import Button from '$lib/shared/ui/Button.svelte';

	const store = setTokensContext();

	let label = $state('');
	let scope = $state<TokenScope>('read');

	onMount(() => {
		void store.load();
	});

	function createToken(event: SubmitEvent): void {
		event.preventDefault();
		const trimmed = label.trim();
		if (!trimmed) return;
		void store.create(trimmed, scope).then(() => {
			label = '';
		});
	}
</script>

<section class="w-full max-w-2xl">
	<h2 class="text-lg font-semibold tracking-tight">{m['profile.tokens.heading']()}</h2>

	<form class="mt-4 flex flex-wrap items-end gap-3" onsubmit={createToken}>
		<label class="grid gap-1 text-sm font-medium">
			{m['profile.tokens.label']()}
			<input class="rounded-md border border-input bg-background px-3 py-2" bind:value={label} required />
		</label>
		<label class="grid gap-1 text-sm font-medium">
			{m['profile.tokens.scope']()}
			<select class="rounded-md border border-input bg-background px-3 py-2" bind:value={scope}>
				<option value="read">{m['profile.tokens.scopeRead']()}</option>
				<option value="read_write">{m['profile.tokens.scopeReadWrite']()}</option>
			</select>
		</label>
		<Button type="submit" loading={store.creating}>{m['profile.tokens.createButton']()}</Button>
	</form>

	{#if store.error}<p role="alert" class="mt-3 text-sm text-destructive">{store.error}</p>{/if}

	{#if store.lastCreatedValue}
		<div role="status" class="mt-3 rounded-md border border-border bg-card p-3 text-sm">
			<p class="font-medium">{m['profile.tokens.copyNow']()}</p>
			<code class="mt-1 block break-all rounded bg-muted px-2 py-1">{store.lastCreatedValue}</code>
			<button class="mt-2 text-sm underline" onclick={() => store.dismissCreatedValue()}>{m['common.dismiss']()}</button>
		</div>
	{/if}

	{#if store.tokens.length === 0 && !store.loading}
		<EmptyState title={m['profile.tokens.empty']()} class="mt-6" />
	{:else}
		<table class="mt-4 w-full border-collapse text-left text-sm">
			<caption class="sr-only">{m['profile.tokens.caption']()}</caption>
			<thead>
				<tr class="border-b border-border">
					<th scope="col" class="px-3 py-2 font-medium">{m['profile.tokens.columnLabel']()}</th>
					<th scope="col" class="px-3 py-2 font-medium">{m['profile.tokens.columnScope']()}</th>
					<th scope="col" class="px-3 py-2 font-medium">{m['profile.tokens.columnCreated']()}</th>
					<th scope="col" class="px-3 py-2 font-medium">{m['profile.tokens.columnLastUsed']()}</th>
					<th scope="col" class="px-3 py-2 font-medium"><span class="sr-only">{m['common.actions']()}</span></th>
				</tr>
			</thead>
			<tbody>
				{#each store.tokens as token (token.id)}
					<tr class="border-b border-border last:border-0">
						<td class="px-3 py-2 font-medium">{token.label}</td>
						<td class="px-3 py-2 text-muted-foreground">{token.scope}</td>
						<td class="px-3 py-2 text-muted-foreground">{new Date(token.createdAt).toLocaleString()}</td>
						<td class="px-3 py-2 text-muted-foreground">{token.lastUsedAt ? new Date(token.lastUsedAt).toLocaleString() : m['profile.tokens.never']()}</td>
						<td class="px-3 py-2 text-right">
							<Button
							variant="ghost"
							size="sm"
							loading={store.revoking[token.id] === true}
							onclick={() => store.revoke(token.id)}
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
