<script lang="ts">
	import { onMount } from 'svelte';
	import { goto } from '$app/navigation';
	import { resolve } from '$app/paths';
	import HomeCapabilities from '#lib/features/home/HomeCapabilities.svelte';
	import HomeClosing from '#lib/features/home/HomeClosing.svelte';
	import HomeGuardrails from '#lib/features/home/HomeGuardrails.svelte';
	import HomeHero from '#lib/features/home/HomeHero.svelte';
	import HomeHowItWorks from '#lib/features/home/HomeHowItWorks.svelte';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';

	const session = getSessionContext();

	// Signed-in users never land on the pitch: admins go to their dashboard,
	// pool users to their machines - the list is the workspace landing page
	// (DESIGN.md §5, "The machine list is the landing page").
	onMount(() => {
		if (session.principal?.isAdmin) {
			void goto(resolve('/admin'));
		} else if (session.principal) {
			void goto(resolve('/vms'), { replace: true });
		}
	});
</script>

{#if !session.principal}
	<!-- The product pitch, a scroll page: hero, how it works, the capability
	     rack, the guardrails, a closing call to action. Anonymous visitors
	     only - a signed-in user is sent to the workspace. -->
	<!-- overflow-x-clip (not hidden): hides the blades' slide-in offset
	     without becoming a scroll container, so the sticky rack still sticks. -->
	<div class="w-full self-stretch overflow-x-clip">
		<HomeHero />
		<HomeHowItWorks />
		<HomeCapabilities />
		<HomeGuardrails />
		<HomeClosing />
	</div>
{/if}
