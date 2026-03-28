<script lang="ts">
	import { t } from 'svelte-i18n';
	import { auth } from '$lib/stores/auth.svelte';
	import { Button } from '$lib/components/ui/button';
	import { Desktop, PlusSquare, ShieldCheck, SignIn, BookOpen } from 'phosphor-svelte';
</script>

<div class="mx-auto max-w-4xl px-6 py-16 md:py-24">

	<!-- Hero -->
	<div class="mb-16 text-center">
		<div class="pv-navbar-logo mx-auto mb-6 h-16 w-16 rounded-2xl text-2xl font-black">PV</div>
		<h1 class="mb-4 text-4xl font-bold tracking-tight md:text-5xl">
			{$t('landing.headline')}
		</h1>
		<p class="text-muted-foreground mx-auto mb-8 max-w-xl text-lg leading-relaxed">
			{$t('landing.subheadline')}
		</p>

		{#if auth.initialized && auth.username}
			<!-- Authenticated: show personalised CTA -->
			<div class="flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
				<Button href="/home" size="lg">
					<Desktop class="mr-2 h-5 w-5" />
					{$t('landing.goToMyVms')}
				</Button>
				<Button href="/docs/user" variant="outline" size="lg">
					<BookOpen class="mr-2 h-5 w-5" />
					{$t('landing.documentation')}
				</Button>
			</div>
			<p class="text-muted-foreground mt-4 text-sm">
				{$t('landing.welcomeBack', { values: { username: auth.username } })}
			</p>
		{:else}
			<!-- Unauthenticated: sign in + docs -->
			<div class="flex flex-col items-center gap-3 sm:flex-row sm:justify-center">
				<Button href="/login" size="lg">
					<SignIn class="mr-2 h-5 w-5" />
					{$t('landing.signIn')}
				</Button>
				<Button href="/docs/user" variant="outline" size="lg">
					<BookOpen class="mr-2 h-5 w-5" />
					{$t('landing.documentation')}
				</Button>
			</div>
		{/if}
	</div>

	<!-- Feature cards -->
	<div class="grid gap-6 md:grid-cols-3">
		<div class="pv-table-wrap p-6">
			<div class="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-orange-100 dark:bg-orange-950/50">
				<Desktop class="h-5 w-5 text-orange-600 dark:text-orange-400" />
			</div>
			<h3 class="mb-2 font-semibold">{$t('landing.feature1Title')}</h3>
			<p class="text-muted-foreground text-sm leading-relaxed">{$t('landing.feature1Desc')}</p>
		</div>

		<div class="pv-table-wrap p-6">
			<div class="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-orange-100 dark:bg-orange-950/50">
				<PlusSquare class="h-5 w-5 text-orange-600 dark:text-orange-400" />
			</div>
			<h3 class="mb-2 font-semibold">{$t('landing.feature2Title')}</h3>
			<p class="text-muted-foreground text-sm leading-relaxed">{$t('landing.feature2Desc')}</p>
		</div>

		<div class="pv-table-wrap p-6">
			<div class="mb-3 flex h-10 w-10 items-center justify-center rounded-lg bg-orange-100 dark:bg-orange-950/50">
				<ShieldCheck class="h-5 w-5 text-orange-600 dark:text-orange-400" />
			</div>
			<h3 class="mb-2 font-semibold">{$t('landing.feature3Title')}</h3>
			<p class="text-muted-foreground text-sm leading-relaxed">{$t('landing.feature3Desc')}</p>
		</div>
	</div>

</div>
