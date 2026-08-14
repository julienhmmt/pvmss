<script lang="ts">
	import TokenList from '$lib/features/auth/TokenList.svelte';
	import { m } from '$lib/paraglide/messages.js';
</script>

<svelte:head>
	<title>{m['profile.tokens.title']()}</title>
</svelte:head>

<section class="mx-auto w-full max-w-4xl px-4 py-8">
	<TokenList />
</section>
