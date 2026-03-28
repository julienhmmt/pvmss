<script lang="ts">
	import { t } from 'svelte-i18n';
	import { auth } from '$lib/stores/auth.svelte';
	import { login, adminLogin, proxmoxAdminLogin } from '$lib/api/auth';
	import { ApiRequestError } from '$lib/types/api';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import * as Card from '$lib/components/ui/card';
	import * as Tabs from '$lib/components/ui/tabs';
	import { UserIcon, ShieldCheckIcon, DesktopIcon, EyeIcon, EyeSlashIcon, EyeSlash, Eye, Desktop } from 'phosphor-svelte';

	type AdminTab = 'local' | 'pve';

	let activeTab = $state<'user' | 'admin'>('user');
	let activeAdminTab = $state<AdminTab>('local');

	// User login
	let userUsername = $state('');
	let userPassword = $state('');
	let userShowPassword = $state(false);

	// Local admin login
	let localPassword = $state('');
	let localShowPassword = $state(false);

	// PVE admin login
	let pveUsername = $state('');
	let pvePassword = $state('');
	let pveShowPassword = $state(false);

	let loading = $state(false);
	let error = $state<string | null>(null);

	function getErrorMessage(e: unknown): string {
		if (e instanceof ApiRequestError) {
			if (e.status === 401) return $t('login.error.invalidCredentials');
			if (e.status >= 500) return $t('login.error.serviceUnavailable');
		}
		return $t('login.error.unknown');
	}

	function addRealmIfMissing(username: string): string {
		if (username && !username.includes('@')) {
			return username + '@pve';
		}
		return username;
	}

	async function handleUserLogin(e: Event) {
		e.preventDefault();
		if (!userUsername || !userPassword) return;
		loading = true;
		error = null;
		try {
			const user = await login(userUsername, userPassword);
			auth.setUser(user.username, user.is_admin);
			window.location.href = '/';
		} catch (err) {
			error = getErrorMessage(err);
		} finally {
			loading = false;
		}
	}

	async function handleLocalAdminLogin(e: Event) {
		e.preventDefault();
		if (!localPassword) return;
		loading = true;
		error = null;
		try {
			const user = await adminLogin(localPassword);
			auth.setUser(user.username, user.is_admin);
			window.location.href = '/admin/';
		} catch (err) {
			error = getErrorMessage(err);
		} finally {
			loading = false;
		}
	}

	async function handlePveAdminLogin(e: Event) {
		e.preventDefault();
		const username = addRealmIfMissing(pveUsername);
		if (!username || !pvePassword) return;
		loading = true;
		error = null;
		try {
			const user = await proxmoxAdminLogin(username, pvePassword);
			auth.setUser(user.username, user.is_admin);
			window.location.href = '/admin/';
		} catch (err) {
			error = getErrorMessage(err);
		} finally {
			loading = false;
		}
	}

	function clearError() {
		error = null;
	}
</script>

<div class="pv-login-bg">
	<div class="pv-login-wrap">
		<!-- Brand -->
		<div class="pv-login-brand">
			<div class="pv-login-logo">PV</div>
			<div>
				<p class="pv-login-brand-name">PVMSS</p>
				<p class="pv-login-brand-sub">{$t('login.subtitle')}</p>
			</div>
			<a href="/" class="pv-login-home-btn" aria-label={$t('login.backHome')}>
				<svg xmlns="http://www.w3.org/2000/svg" width="16" height="16" fill="currentColor" viewBox="0 0 256 256"><path d="M224,115.55V208a16,16,0,0,1-16,16H168a16,16,0,0,1-16-16V168a8,8,0,0,0-8-8H112a8,8,0,0,0-8,8v40a16,16,0,0,1-16,16H48a16,16,0,0,1-16-16V115.55a16,16,0,0,1,5.17-11.78l80-75.48.11-.11a16,16,0,0,1,21.53,0,1.14,1.14,0,0,0,.11.11l80,75.48A16,16,0,0,1,224,115.55Z"></path></svg>
				{$t('login.backHome')}
			</a>
		</div>

		<Card.Root class="pv-login-card">
			<Card.Header class="pb-4">
				<Card.Title class="pv-login-title">{$t('login.title')}</Card.Title>
			</Card.Header>
			<Card.Content>
				<!-- Error banner -->
				{#if error}
					<div class="pv-login-error" role="alert">
						<span>{error}</span>
						<button class="pv-login-error-close" onclick={clearError} aria-label="Dismiss">✕</button>
					</div>
				{/if}

				<!-- Top-level tabs: User / Admin -->
				<Tabs.Root
					value={activeTab}
					onValueChange={(v) => {
						activeTab = v as 'user' | 'admin';
						clearError();
					}}
				>
					<Tabs.List class="pv-login-tabs-list">
						<Tabs.Trigger value="user" class="pv-login-tab">
							<UserIcon class="mr-1.5 h-4 w-4" />
							{$t('login.userTab')}
						</Tabs.Trigger>
						<Tabs.Trigger value="admin" class="pv-login-tab">
							<ShieldCheckIcon class="mr-1.5 h-4 w-4" />
							{$t('login.adminTab')}
						</Tabs.Trigger>
					</Tabs.List>

					<!-- User login -->
					<Tabs.Content value="user" class="mt-5">
						<p class="pv-login-hint">{$t('login.userHint')}</p>
						<form onsubmit={handleUserLogin} novalidate class="space-y-4">
							<div class="space-y-1.5">
								<Label for="user-username">{$t('login.username')}</Label>
								<Input
									id="user-username"
									type="text"
									bind:value={userUsername}
									placeholder={$t('login.usernamePlaceholder')}
									autocomplete="username"
									required
									minlength={3}
									maxlength={50}
									disabled={loading}
								/>
							</div>
							<div class="space-y-1.5">
								<Label for="user-password">{$t('login.password')}</Label>
								<div class="relative">
									<Input
										id="user-password"
										type={userShowPassword ? 'text' : 'password'}
										bind:value={userPassword}
										placeholder={$t('login.passwordPlaceholder')}
										autocomplete="current-password"
										required
										minlength={6}
										maxlength={128}
										disabled={loading}
										class="pr-10"
									/>
									<button
										type="button"
										class="pv-login-EyeIcon"
										onclick={() => (userShowPassword = !userShowPassword)}
										aria-label="Toggle password visibility"
									>
										{#if userShowPassword}
											<EyeSlashIcon class="h-4 w-4" />
										{:else}
											<EyeIcon class="h-4 w-4" />
										{/if}
									</button>
								</div>
							</div>
							<Button type="submit" class="w-full" disabled={loading || !userUsername || !userPassword}>
								{loading ? $t('login.signingIn') : $t('login.signIn')}
							</Button>
						</form>
					</Tabs.Content>

					<!-- Admin login -->
					<Tabs.Content value="admin" class="mt-5">
						<Tabs.Root
							value={activeAdminTab}
							onValueChange={(v) => {
								activeAdminTab = v as AdminTab;
								clearError();
							}}
						>
							<Tabs.List class="pv-login-tabs-list pv-login-tabs-list--inner">
								<Tabs.Trigger value="local" class="pv-login-tab">
									<ShieldCheckIcon class="mr-1.5 h-3.5 w-3.5" />
									{$t('login.localAdminTab')}
								</Tabs.Trigger>
								<Tabs.Trigger value="pve" class="pv-login-tab">
									<DesktopIcon class="mr-1.5 h-3.5 w-3.5" />
									{$t('login.pveAdminTab')}
								</Tabs.Trigger>
							</Tabs.List>

							<!-- Local PVMSS admin -->
							<Tabs.Content value="local" class="mt-4">
								<p class="pv-login-hint">{$t('login.localAdminHint')}</p>
								<form onsubmit={handleLocalAdminLogin} novalidate class="space-y-4">
									<div class="space-y-1.5">
										<Label for="local-password">{$t('login.password')}</Label>
										<div class="relative">
											<Input
												id="local-password"
												type={localShowPassword ? 'text' : 'password'}
												bind:value={localPassword}
												placeholder={$t('login.adminPasswordPlaceholder')}
												autocomplete="current-password"
												required
												minlength={6}
												maxlength={128}
												disabled={loading}
												class="pr-10"
											/>
											<button
												type="button"
												class="pv-login-EyeIcon"
												onclick={() => (localShowPassword = !localShowPassword)}
												aria-label="Toggle password visibility"
											>
												{#if localShowPassword}
													<EyeSlash class="h-4 w-4" />
												{:else}
													<Eye class="h-4 w-4" />
												{/if}
											</button>
										</div>
									</div>
									<Button type="submit" class="w-full" disabled={loading || !localPassword}>
										{loading ? $t('login.signingIn') : $t('login.signIn')}
									</Button>
								</form>
							</Tabs.Content>

							<!-- PVE admin -->
							<Tabs.Content value="pve" class="mt-4">
								<p class="pv-login-hint">{$t('login.pveAdminHint')}</p>
								<form onsubmit={handlePveAdminLogin} novalidate class="space-y-4">
									<div class="space-y-1.5">
										<Label for="pve-username">{$t('login.username')}</Label>
										<Input
											id="pve-username"
											type="text"
											bind:value={pveUsername}
											placeholder={$t('login.usernamePlaceholder')}
											autocomplete="username"
											required
											minlength={3}
											maxlength={100}
											disabled={loading}
											onblur={() => (pveUsername = addRealmIfMissing(pveUsername))}
										/>
										<p class="pv-login-realm-hint">{$t('login.addRealm')}</p>
									</div>
									<div class="space-y-1.5">
										<Label for="pve-password">{$t('login.password')}</Label>
										<div class="relative">
											<Input
												id="pve-password"
												type={pveShowPassword ? 'text' : 'password'}
												bind:value={pvePassword}
												placeholder={$t('login.passwordPlaceholder')}
												autocomplete="current-password"
												required
												minlength={6}
												maxlength={128}
												disabled={loading}
												class="pr-10"
											/>
											<button
												type="button"
												class="pv-login-EyeIcon"
												onclick={() => (pveShowPassword = !pveShowPassword)}
												aria-label="Toggle password visibility"
											>
												{#if pveShowPassword}
													<EyeSlash class="h-4 w-4" />
												{:else}
													<Eye class="h-4 w-4" />
												{/if}
											</button>
										</div>
									</div>
									<Button
										type="submit"
										class="w-full"
										disabled={loading || !pveUsername || !pvePassword}
									>
										{loading ? $t('login.signingIn') : $t('login.signIn')}
									</Button>
								</form>
							</Tabs.Content>
						</Tabs.Root>
					</Tabs.Content>
				</Tabs.Root>
			</Card.Content>
		</Card.Root>
	</div>
</div>

<style>
	.pv-login-bg {
		min-height: 100vh;
		display: flex;
		align-items: center;
		justify-content: center;
		padding: 1.5rem;
		background: var(--background);
	}

	.pv-login-wrap {
		width: 100%;
		max-width: 420px;
		display: flex;
		flex-direction: column;
		gap: 1.5rem;
	}

	.pv-login-brand {
		display: flex;
		align-items: center;
		gap: 0.875rem;
		justify-content: space-between;
	}

	.pv-login-home-btn {
		display: flex;
		align-items: center;
		gap: 0.375rem;
		font-size: 0.8125rem;
		font-weight: 500;
		color: var(--muted-foreground);
		text-decoration: none;
		padding: 0.375rem 0.625rem;
		border-radius: var(--radius);
		transition: background 0.12s, color 0.12s;
		margin-left: auto;
		flex-shrink: 0;
	}

	.pv-login-home-btn:hover {
		background: var(--accent);
		color: var(--accent-foreground);
	}

	.pv-login-logo {
		width: 2.75rem;
		height: 2.75rem;
		border-radius: var(--radius);
		background: var(--primary);
		color: var(--primary-foreground);
		font-weight: 800;
		font-size: 1rem;
		display: flex;
		align-items: center;
		justify-content: center;
		letter-spacing: -0.02em;
		flex-shrink: 0;
	}

	.pv-login-brand-name {
		font-size: 1.375rem;
		font-weight: 700;
		color: var(--foreground);
		line-height: 1.2;
	}

	.pv-login-brand-sub {
		font-size: 0.75rem;
		color: var(--muted-foreground);
		line-height: 1.3;
	}

	:global(.pv-login-card) {
		border: 1px solid var(--border);
		box-shadow:
			0 4px 6px -1px oklch(0 0 0 / 0.07),
			0 2px 4px -2px oklch(0 0 0 / 0.05);
	}

	:global(.pv-login-title) {
		font-size: 1.125rem;
		font-weight: 600;
	}

	:global(.pv-login-tabs-list) {
		width: 100%;
		display: grid;
		grid-template-columns: 1fr 1fr;
	}

	:global(.pv-login-tabs-list--inner) {
		background: oklch(from var(--muted) l c h / 0.5);
	}

	:global(.pv-login-tab) {
		display: flex;
		align-items: center;
		justify-content: center;
		font-size: 0.875rem;
	}

	.pv-login-error {
		display: flex;
		align-items: center;
		justify-content: space-between;
		gap: 0.5rem;
		padding: 0.625rem 0.875rem;
		border-radius: var(--radius);
		background: oklch(from var(--destructive) l c h / 0.1);
		border: 1px solid oklch(from var(--destructive) l c h / 0.25);
		color: var(--destructive);
		font-size: 0.875rem;
		margin-bottom: 1rem;
	}

	.pv-login-error-close {
		background: none;
		border: none;
		cursor: pointer;
		color: var(--destructive);
		opacity: 0.7;
		padding: 0;
		line-height: 1;
		flex-shrink: 0;
	}

	.pv-login-error-close:hover {
		opacity: 1;
	}

	.pv-login-hint {
		font-size: 0.8125rem;
		color: var(--muted-foreground);
		margin-bottom: 1rem;
	}

	.pv-login-realm-hint {
		font-size: 0.75rem;
		color: var(--muted-foreground);
		margin-top: 0.25rem;
	}

	.pv-login-EyeIcon {
		position: absolute;
		right: 0.625rem;
		top: 50%;
		transform: translateY(-50%);
		background: none;
		border: none;
		cursor: pointer;
		color: var(--muted-foreground);
		padding: 0;
		display: flex;
		align-items: center;
	}

	.pv-login-EyeIcon:hover {
		color: var(--foreground);
	}
</style>
