<script lang="ts">
	import { t } from 'svelte-i18n';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import { Switch } from '$lib/components/ui/switch';
	import { Textarea } from '$lib/components/ui/textarea';
	import * as Select from '$lib/components/ui/select';
	import type { VMCreateSettings } from '$lib/types/vm-create';
	import type { VMCreateFormStore } from '$lib/stores/vm-create.svelte';

	interface Props {
		store: VMCreateFormStore;
		settings: VMCreateSettings;
	}

	let { store, settings }: Props = $props();
</script>

<h2 class="text-lg font-semibold">{$t('vmCreate.cloudinit.title')}</h2>
<Separator />
<div class="flex items-center gap-3">
	<Switch bind:checked={store.form.cloudInitEnabled} />
	<span class="text-sm font-medium">{$t('vmCreate.cloudinit.enable')}</span>
</div>
{#if store.form.cloudInitEnabled}
	<div class="grid gap-4 sm:grid-cols-2">
		{#if settings.cloudinitTemplates.length > 0}
			<div class="space-y-2 sm:col-span-2">
				<Label>{$t('vmCreate.cloudinit.template')}</Label>
				<Select.Root
					type="single"
					value={store.form.ciTemplateID}
					onValueChange={(v) => (store.form.ciTemplateID = v)}
				>
					<Select.Trigger class="w-full">
						{#if store.form.ciTemplateID}
							{settings.cloudinitTemplates.find((t) => t.id === store.form.ciTemplateID)
								?.name || store.form.ciTemplateID}
						{:else}
							{$t('vmCreate.cloudinit.noTemplate')}
						{/if}
					</Select.Trigger>
					<Select.Content>
						<Select.Item value=""
							>{$t('vmCreate.cloudinit.noTemplate')}</Select.Item
						>
						{#each settings.cloudinitTemplates as tpl, i (i)}
							<Select.Item value={tpl.id}>
								{tpl.name}
								{#if tpl.description}
									<span class="text-muted-foreground text-xs">
										— {tpl.description}</span
									>
								{/if}
							</Select.Item>
						{/each}
					</Select.Content>
				</Select.Root>
				{#if !settings.cloudInitSftpEnabled && store.form.ciTemplateID}
					<p class="rounded-md border border-yellow-500/30 bg-yellow-500/5 px-3 py-2 text-xs text-yellow-700 dark:text-yellow-400">
						{$t('vmCreate.cloudinit.templateSftpWarning')}
					</p>
				{/if}
			</div>
		{/if}
		<div class="space-y-2">
			<Label for="ci-user">{$t('vmCreate.cloudinit.user')}</Label>
			<Input
				id="ci-user"
				bind:value={store.form.ciUser}
				placeholder={$t('vmCreate.cloudinit.userPlaceholder')}
			/>
		</div>
		<div class="space-y-2">
			<Label for="ci-password">{$t('vmCreate.cloudinit.password')}</Label>
			<Input
				id="ci-password"
				type="password"
				bind:value={store.form.ciPassword}
				placeholder={$t('vmCreate.cloudinit.passwordPlaceholder')}
			/>
		</div>
		<div class="space-y-2 sm:col-span-2">
			<Label for="ci-ssh">{$t('vmCreate.cloudinit.sshKeys')}</Label>
			<Textarea
				id="ci-ssh"
				bind:value={store.form.ciSSHKeys}
				placeholder={$t('vmCreate.cloudinit.sshKeysPlaceholder')}
				rows={3}
			/>
		</div>
		<div class="space-y-2 sm:col-span-2">
			<Label>{$t('vmCreate.cloudinit.ipConfig')}</Label>
			<div class="flex gap-3">
				<button
					type="button"
					onclick={() => (store.form.ciIPConfig = 'dhcp')}
					class="flex-1 rounded-lg border-2 p-3 text-center text-sm transition-colors
						{store.form.ciIPConfig === 'dhcp'
						? 'border-primary bg-primary/5'
						: 'border-muted hover:border-muted-foreground/30'}"
				>
					{$t('vmCreate.cloudinit.dhcp')}
				</button>
				<button
					type="button"
					onclick={() => (store.form.ciIPConfig = 'static')}
					class="flex-1 rounded-lg border-2 p-3 text-center text-sm transition-colors
						{store.form.ciIPConfig === 'static'
						? 'border-primary bg-primary/5'
						: 'border-muted hover:border-muted-foreground/30'}"
				>
					{$t('vmCreate.cloudinit.static')}
				</button>
			</div>
		</div>
		{#if store.form.ciIPConfig === 'static'}
			<div class="space-y-2">
				<Label for="ci-ip">{$t('vmCreate.cloudinit.ip')}</Label>
				<Input
					id="ci-ip"
					bind:value={store.form.ciIP}
					placeholder={$t('vmCreate.cloudinit.ipPlaceholder')}
				/>
			</div>
			<div class="space-y-2">
				<Label for="ci-gw">{$t('vmCreate.cloudinit.gateway')}</Label>
				<Input
					id="ci-gw"
					bind:value={store.form.ciGateway}
					placeholder={$t('vmCreate.cloudinit.gatewayPlaceholder')}
				/>
			</div>
		{/if}
		<div class="space-y-2">
			<Label for="ci-dns">{$t('vmCreate.cloudinit.dns')}</Label>
			<Input
				id="ci-dns"
				bind:value={store.form.ciDNS}
				placeholder={$t('vmCreate.cloudinit.dnsPlaceholder')}
			/>
		</div>
	</div>
{/if}
