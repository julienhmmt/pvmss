<script lang="ts">
	import { goto } from '$app/navigation';
	import { auth } from '$lib/stores/auth.svelte';

	let { children } = $props();

	$effect(() => {
		if (auth.initialized && !auth.isAdmin) {
			goto('/');
		}
	});
</script>

{#if !auth.initialized}
	<div class="auth-loading">
		<div class="auth-loading-spinner"></div>
	</div>
{:else if auth.isAdmin}
	{@render children()}
{/if}

<style>
	.auth-loading {
		display: flex;
		align-items: center;
		justify-content: center;
		min-height: 50vh;
	}

	.auth-loading-spinner {
		width: 2rem;
		height: 2rem;
		border: 3px solid var(--border);
		border-top-color: var(--primary);
		border-radius: 50%;
		animation: spin 0.8s linear infinite;
	}

	@keyframes spin {
		to {
			transform: rotate(360deg);
		}
	}
</style>
