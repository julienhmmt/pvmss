<script lang="ts">
	import { onMount } from 'svelte';
	import { setTokensContext, type TokenScope } from './tokens.svelte';

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
	<h2 class="text-lg font-semibold tracking-tight">API tokens</h2>

	<form class="mt-4 flex flex-wrap items-end gap-3" onsubmit={createToken}>
		<label class="grid gap-1 text-sm font-medium">
			Label
			<input class="rounded-md border border-input bg-background px-3 py-2" bind:value={label} required />
		</label>
		<label class="grid gap-1 text-sm font-medium">
			Scope
			<select class="rounded-md border border-input bg-background px-3 py-2" bind:value={scope}>
				<option value="read">Read</option>
				<option value="read_write">Read/write</option>
			</select>
		</label>
		<button class="rounded-md bg-primary px-3 py-2 font-medium text-primary-foreground" type="submit">Create token</button>
	</form>

	{#if store.error}<p role="alert" class="mt-3 text-sm text-destructive">{store.error}</p>{/if}

	{#if store.lastCreatedValue}
		<div role="status" class="mt-3 rounded-md border border-border bg-card p-3 text-sm">
			<p class="font-medium">Copy this token now — it will not be shown again.</p>
			<code class="mt-1 block break-all rounded bg-muted px-2 py-1">{store.lastCreatedValue}</code>
			<button class="mt-2 text-sm underline" onclick={() => store.dismissCreatedValue()}>Dismiss</button>
		</div>
	{/if}

	<table class="mt-4 w-full border-collapse text-left text-sm">
		<caption class="sr-only">API tokens</caption>
		<thead>
			<tr class="border-b border-border">
				<th scope="col" class="px-3 py-2 font-medium">Label</th>
				<th scope="col" class="px-3 py-2 font-medium">Scope</th>
				<th scope="col" class="px-3 py-2 font-medium">Created</th>
				<th scope="col" class="px-3 py-2 font-medium">Last used</th>
				<th scope="col" class="px-3 py-2 font-medium"><span class="sr-only">Actions</span></th>
			</tr>
		</thead>
		<tbody>
			{#each store.tokens as token (token.id)}
				<tr class="border-b border-border last:border-0">
					<td class="px-3 py-2 font-medium">{token.label}</td>
					<td class="px-3 py-2 text-muted-foreground">{token.scope}</td>
					<td class="px-3 py-2 text-muted-foreground">{new Date(token.createdAt).toLocaleString()}</td>
					<td class="px-3 py-2 text-muted-foreground">{token.lastUsedAt ? new Date(token.lastUsedAt).toLocaleString() : 'never'}</td>
					<td class="px-3 py-2 text-right">
						<button class="text-sm text-destructive underline" onclick={() => store.revoke(token.id)}>Revoke</button>
					</td>
				</tr>
			{/each}
		</tbody>
	</table>
</section>
