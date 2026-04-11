<script lang="ts">
	import { t } from 'svelte-i18n';
	import { auth } from '$lib/stores/auth.svelte';
	import { GearSixIcon, SignOutIcon, CaretDownIcon } from 'phosphor-svelte';

	interface Props {
		onLogout: () => Promise<void>;
		onNavigate: (url: string) => void;
		onClose: () => void;
	}

	let { onLogout, onNavigate, onClose }: Props = $props();
</script>

<div class="pv-user-menu">
	<div class="pv-user-menu-label">
		<p class="pv-user-menu-username">{auth.username}</p>
		<p class="pv-user-menu-role">{auth.isAdmin ? $t('nav.administrator') : $t('nav.user')}</p>
	</div>
	<div class="pv-user-menu-divider"></div>
	{#if auth.isAdmin}
		<button
			class="pv-user-menu-item"
			onclick={() => {
				onNavigate('/admin/');
				onClose();
			}}
		>
			<GearSixIcon class="h-4 w-4" />
			{$t('common.admin')}
		</button>
		<div class="pv-user-menu-divider"></div>
	{/if}
	<button
		class="pv-user-menu-item pv-user-menu-item--danger"
		onclick={onLogout}
	>
		<SignOutIcon class="h-4 w-4" />
		{$t('common.logout')}
	</button>
</div>

<style>
	.pv-user-menu {
		display: flex;
		flex-direction: column;
		gap: 4px;
		padding: 4px;
		min-width: 200px;
	}

	.pv-user-menu-label {
		display: flex;
		flex-direction: column;
		gap: 2px;
		padding: 8px 12px;
		font-weight: normal;
	}

	.pv-user-menu-username {
		font-size: 0.875rem;
		font-weight: 600;
		line-height: 1;
		color: var(--foreground);
	}

	.pv-user-menu-role {
		font-size: 0.75rem;
		line-height: 1;
		color: var(--muted-foreground);
	}

	.pv-user-menu-divider {
		height: 1px;
		background: var(--border);
		margin: 4px 0;
	}

	.pv-user-menu-item {
		display: flex;
		align-items: center;
		gap: 8px;
		padding: 8px 12px;
		border-radius: 6px;
		font-size: 0.875rem;
		font-weight: 500;
		color: var(--foreground);
		background: transparent;
		border: none;
		cursor: pointer;
		transition: background 0.12s;
		width: 100%;
		text-align: left;
	}

	.pv-user-menu-item:hover {
		background: var(--accent);
	}

	.pv-user-menu-item--danger {
		color: hsl(var(--destructive));
	}

	.pv-user-menu-item--danger:hover {
		background: hsl(var(--destructive) / 0.1);
		color: hsl(var(--destructive));
	}
</style>
