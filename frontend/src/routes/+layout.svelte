<script lang="ts">
	import { onMount } from 'svelte';
	import { page } from '$app/stores';
	import { auth } from '$lib/stores/auth.svelte';
	import { themeStore } from '$lib/stores/theme.svelte';
	import { Toaster } from '$lib/components/ui/sonner';
	import Navbar from '$lib/components/layout/Navbar.svelte';
	import Footer from '$lib/components/layout/Footer.svelte';
	import '$lib/i18n';
	import '../app.css';
	import { getSetupStatus } from '$lib/api/setup';

	let { children } = $props();

	const noShellPaths = ['/login', '/setup'];
	function isShellHidden(pathname: string): boolean {
		return noShellPaths.some(p => pathname === p || pathname.startsWith(p + '/'))
			|| pathname.endsWith('/console');
	}

	onMount(() => {
		themeStore.init();
		auth.exchange().catch(err => {
			console.error('Auth exchange failed:', err);
		});
		// T129: redirect to /setup when bootstrap is not yet complete
		const pathname = window.location.pathname;
		if (pathname !== '/setup' && !pathname.startsWith('/setup/')) {
			getSetupStatus().then(status => {
				if (!status.complete) {
					window.location.href = '/setup';
				}
			}).catch(() => {
				// Network error — let the page load normally
			});
		}
	});
</script>

{#if !isShellHidden($page.url.pathname)}
	<Navbar />
	<div class="pt-14 flex flex-col flex-1">
		{@render children()}
	</div>
	<Footer />
{:else}
	{@render children()}
{/if}
<Toaster richColors closeButton position="top-right" />
