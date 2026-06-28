<script lang="ts">
	import { untrack } from 'svelte';
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { CloudArrowUp, FileCode } from 'phosphor-svelte';
	import { Button } from '$lib/components/ui/button';
	import { Input } from '$lib/components/ui/input';
	import { Label } from '$lib/components/ui/label';
	import { Separator } from '$lib/components/ui/separator';
	import { Textarea } from '$lib/components/ui/textarea';
	import { ApiRequestError } from '$lib/types/api';
	import {
		type CloudInitUpdatePayload,
		type VMConfig,
		getVMCloudInitSnippet,
		updateVMCloudInit,
		updateVMCloudInitSnippet,
	} from '$lib/api/vm-details';

	interface Props {
		config: VMConfig;
		/** Called after a successful save so the parent can refresh config. */
		onSaved?: () => void;
	}

	let { config, onSaved }: Props = $props();

	type IPMode = 'dhcp' | 'static';

	interface CloudInitFormState {
		user: string;
		password: string;
		sshKeys: string;
		ipMode: IPMode;
		ip: string;
		gateway: string;
		isIPv6: boolean;
		dhcpToken: string;
		nameserver: string;
		searchdomain: string;
	}

	/** Whether cloud-init is configured at all (ciuser/sshkeys/ipconfig/nameserver/searchdomain). */
	const hasCloudInit = $derived(!!config.cloudInit);

	/**
	 * Build the initial form state from the live config. Returns a fresh object
	 * each time config changes so the form tracks the latest server state.
	 */
	function buildInitialState(cfg: VMConfig): CloudInitFormState {
		const ci = cfg.cloudInit;
		const ipConfig = ci?.ipConfig ?? '';
		let ipMode: IPMode = 'dhcp';
		let ip = '';
		let gateway = '';
		let isIPv6 = false;
		let dhcpToken = 'ip=dhcp';
		if (ipConfig) {
			for (const tok of ipConfig.split(',')) {
				const trimmed = tok.trim();
				if (trimmed === 'ip=dhcp') {
					ipMode = 'dhcp';
					dhcpToken = 'ip=dhcp';
				} else if (trimmed === 'ip6=auto' || trimmed === 'ip6=dhcp') {
					ipMode = 'dhcp';
					isIPv6 = true;
					dhcpToken = trimmed;
				} else if (trimmed.startsWith('ip6=')) {
					ipMode = 'static';
					isIPv6 = true;
					ip = trimmed.slice(4);
				} else if (trimmed.startsWith('ip=')) {
					ipMode = 'static';
					isIPv6 = false;
					ip = trimmed.slice(3);
				} else if (trimmed.startsWith('gw6=')) {
					gateway = trimmed.slice(4);
					isIPv6 = true;
				} else if (trimmed.startsWith('gw=')) {
					gateway = trimmed.slice(3);
				}
			}
		}
		return {
			user: ci?.user ?? '',
			password: '',
			sshKeys: ci?.sshKeys ?? '',
			ipMode,
			ip,
			gateway,
			isIPv6,
			dhcpToken,
			nameserver: ci?.nameserver ?? '',
			searchdomain: ci?.searchdomain ?? '',
		};
	}

	// Seed from the initial config without tracking it here; the $effect below
	// keeps `form` in sync when the config reference changes.
	let form = $state<CloudInitFormState>(untrack(() => buildInitialState(config)));
	let saving = $state(false);

	const baseState = $derived(buildInitialState(config));

	// Reset the form whenever the config reference changes (e.g. after a refresh).
	$effect(() => {
		form = buildInitialState(config);
	});

	/** Whether any field has changed relative to the loaded config. */
	const isDirty = $derived.by(() => {
		return (
			form.user !== baseState.user ||
			form.password !== '' ||
			form.sshKeys !== baseState.sshKeys ||
			form.ipMode !== baseState.ipMode ||
			form.ip !== baseState.ip ||
			form.gateway !== baseState.gateway ||
			form.isIPv6 !== baseState.isIPv6 ||
			form.dhcpToken !== baseState.dhcpToken ||
			form.nameserver !== baseState.nameserver ||
			form.searchdomain !== baseState.searchdomain
		);
	});

	const staticIPIncomplete = $derived(
		form.ipMode === 'static' && !form.ip.trim() && !form.gateway.trim(),
	);

	const canSave = $derived(isDirty && !staticIPIncomplete && !saving);

	/** Build the ipconfig0 string from form state. Returns null when static mode
	 *  has no IP or gateway (incomplete form — caller should skip, not clear). */
	function buildIPConfig(
		mode: IPMode,
		ip: string,
		gateway: string,
		isIPv6: boolean,
		dhcpToken: string,
	): string | null {
		const ipPrefix = isIPv6 ? 'ip6' : 'ip';
		const gwPrefix = isIPv6 ? 'gw6' : 'gw';
		if (mode === 'dhcp') {
			let out = dhcpToken;
			if (gateway.trim()) {
				out += `,${gwPrefix}=${gateway.trim()}`;
			}
			return out;
		}
		const parts: string[] = [];
		if (ip.trim()) {
			parts.push(`${ipPrefix}=${ip.trim()}`);
		}
		if (gateway.trim()) {
			parts.push(`${gwPrefix}=${gateway.trim()}`);
		}
		if (parts.length === 0) {
			return null;
		}
		return parts.join(',');
	}

	/** Build the request payload. Only changed fields are included so the
	 *  backend skips unchanged ones (nil pointer = leave unchanged). */
	function buildPayload(): CloudInitUpdatePayload {
		const payload: CloudInitUpdatePayload = {};
		if (form.user.trim() !== baseState.user) {
			payload.user = form.user.trim();
		}
		if (form.password !== '') {
			payload.password = form.password;
		}
		if (form.sshKeys.trim() !== baseState.sshKeys) {
			payload.sshKeys = form.sshKeys.trim();
		}
		const currentIP = buildIPConfig(form.ipMode, form.ip, form.gateway, form.isIPv6, form.dhcpToken);
		const baseIP = buildIPConfig(baseState.ipMode, baseState.ip, baseState.gateway, baseState.isIPv6, baseState.dhcpToken);
		if (currentIP !== baseIP && currentIP !== null) {
			payload.ipConfig = currentIP;
		}
		if (form.nameserver.trim() !== baseState.nameserver) {
			payload.nameserver = form.nameserver.trim();
		}
		if (form.searchdomain.trim() !== baseState.searchdomain) {
			payload.searchdomain = form.searchdomain.trim();
		}
		return payload;
	}

	async function save() {
		if (!config || saving) return;
		saving = true;
		try {
			const payload = buildPayload();
			await updateVMCloudInit(config.vmid, payload);
			toast.success($t('vm.cloudInit.saved'));
			// Clear the one-shot password field so it doesn't linger in the DOM.
			form.password = '';
			onSaved?.();
		} catch (err: unknown) {
			if (err instanceof ApiRequestError && err.error.message) {
				toast.error(err.error.message);
			} else {
				console.error(
					'updateVMCloudInit failed:',
					err instanceof Error ? err.message : String(err),
				);
				toast.error($t('common.error'));
			}
		} finally {
			saving = false;
		}
	}

	function reset() {
		form = buildInitialState(config);
	}

	// ── Custom cloud-config YAML (cicustom snippet) ──────────────────────────
	// The Proxmox HTTP API cannot reliably read/write snippets, so this section
	// is gated on SFTP being configured. When enabled, the snippet is loaded
	// lazily and the user can edit + re-upload it (creating it if missing).

	const sftpEnabled = $derived(!!config.cloudInitSftpEnabled);

	let customYaml = $state('#cloud-config\n');
	let customYamlOriginal = $state('#cloud-config\n');
	let customYamlLoading = $state(false);
	let customYamlSaving = $state(false);
	let customYamlLoadError = $state('');
	// Whether the loaded snippet can be saved (SFTP configured). When false the
	// content is a read-only render of the effective cloud-config (dump).
	let customYamlEditable = $state(false);

	const customYamlDirty = $derived(customYaml !== customYamlOriginal);

	async function loadCustomYaml() {
		if (customYamlLoading || customYamlSaving) return;
		customYamlLoading = true;
		customYamlLoadError = '';
		try {
			const snippet = await getVMCloudInitSnippet(config.vmid);
			const content = snippet.content || '#cloud-config\n';
			customYaml = content;
			customYamlOriginal = content;
			customYamlEditable = snippet.editable;
		} catch (err: unknown) {
			const msg = err instanceof ApiRequestError && err.error.message
				? err.error.message
				: err instanceof Error ? err.message : String(err);
			customYamlLoadError = msg;
		} finally {
			customYamlLoading = false;
		}
	}

	async function saveCustomYaml() {
		if (customYamlSaving || !customYamlDirty) return;
		customYamlSaving = true;
		try {
			const snippet = await updateVMCloudInitSnippet(config.vmid, customYaml);
			const content = snippet.content || customYaml;
			customYaml = content;
			customYamlOriginal = content;
			toast.success($t('vm.cloudInit.customYamlSaved'));
			onSaved?.();
		} catch (err: unknown) {
			if (err instanceof ApiRequestError && err.error.message) {
				toast.error(err.error.message);
			} else {
				console.error(
					'updateVMCloudInitSnippet failed:',
					err instanceof Error ? err.message : String(err),
				);
				toast.error($t('common.error'));
			}
		} finally {
			customYamlSaving = false;
		}
	}

	// Load the snippet on mount. When SFTP is configured the content is the
	// editable per-VM snippet; otherwise it is a read-only render of the
	// effective cloud-config (the backend decides and reports `editable`).
	// Reading `sftpEnabled` here re-triggers the load if an admin toggles SFTP
	// mid-session. We deliberately do NOT depend on the full config object —
	// that would clobber in-progress edits on every refresh. The snippet is
	// otherwise re-fetched only on explicit retry or after a successful save.
	$effect(() => {
		// Track ONLY the VM id and SFTP availability so we reload exactly when
		// one of those changes. loadCustomYaml() itself reads and writes
		// customYaml* state (e.g. customYamlLoading); running it untracked stops
		// those writes from re-triggering this effect, which would otherwise loop
		// forever (load → set loading=false in finally → effect re-runs → load…).
		void config.vmid;
		void sftpEnabled;
		untrack(() => {
			void loadCustomYaml();
		});
	});
</script>

<div class="pv-table-wrap">
	{#if !hasCloudInit && !sftpEnabled}
		<div class="flex flex-col items-center py-12 text-muted-foreground">
			<CloudArrowUp class="mb-3 h-10 w-10 opacity-30" />
			<p class="text-sm">{$t('vm.noCloudInit')}</p>
		</div>
	{:else}
		<div class="flex items-center justify-between border-b border-border px-4 py-3">
			<span class="text-sm font-medium">{$t('vm.tabCloudInit')}</span>
		</div>

		<div class="space-y-5 p-4">
			<p class="rounded-md border border-yellow-500/30 bg-yellow-500/5 px-3 py-2 text-xs text-yellow-700 dark:text-yellow-400">
				{$t('vm.cloudInit.rebootNotice')}
			</p>

			<div class="grid gap-4 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="ci-user">{$t('vm.cloudInit.user')}</Label>
					<Input id="ci-user" bind:value={form.user} placeholder={$t('vm.cloudInit.userPlaceholder')} />
				</div>
				<div class="space-y-2">
					<Label for="ci-password">{$t('vm.cloudInit.password')}</Label>
					<Input
						id="ci-password"
						type="password"
						bind:value={form.password}
						placeholder={$t('vm.cloudInit.passwordPlaceholder')}
						autocomplete="new-password"
					/>
				</div>
			</div>

			<div class="space-y-2">
				<Label for="ci-ssh">{$t('vm.cloudInit.sshKeys')}</Label>
				<Textarea
					id="ci-ssh"
					bind:value={form.sshKeys}
					placeholder={$t('vm.cloudInit.sshKeysPlaceholder')}
					rows={4}
					class="font-mono text-xs"
				/>
			</div>

			<Separator />

			<div class="space-y-2">
				<Label>{$t('vm.cloudInit.ipConfig')}</Label>
				<div class="flex gap-3">
					<button
						type="button"
						onclick={() => (form.ipMode = 'dhcp')}
						class="flex-1 rounded-lg border-2 p-3 text-center text-sm transition-colors
							{form.ipMode === 'dhcp'
							? 'border-primary bg-primary/5'
							: 'border-muted hover:border-muted-foreground/30'}"
					>
						{$t('vm.cloudInit.dhcp')}
					</button>
					<button
						type="button"
						onclick={() => (form.ipMode = 'static')}
						class="flex-1 rounded-lg border-2 p-3 text-center text-sm transition-colors
							{form.ipMode === 'static'
							? 'border-primary bg-primary/5'
							: 'border-muted hover:border-muted-foreground/30'}"
					>
						{$t('vm.cloudInit.static')}
					</button>
				</div>
			</div>

			{#if form.ipMode === 'static'}
				<div class="grid gap-4 sm:grid-cols-2">
					<div class="space-y-2">
						<Label for="ci-ip">{$t('vm.cloudInit.ip')}</Label>
						<Input
							id="ci-ip"
							bind:value={form.ip}
							placeholder={$t('vm.cloudInit.ipPlaceholder')}
						/>
					</div>
					<div class="space-y-2">
						<Label for="ci-gw">{$t('vm.cloudInit.gateway')}</Label>
						<Input
							id="ci-gw"
							bind:value={form.gateway}
							placeholder={$t('vm.cloudInit.gatewayPlaceholder')}
						/>
					</div>
				</div>
				{#if staticIPIncomplete}
					<p class="text-xs text-destructive">{$t('vm.cloudInit.ipRequired')}</p>
				{/if}
			{:else}
				<div class="space-y-2">
					<Label for="ci-gw-dhcp">{$t('vm.cloudInit.gatewayOptional')}</Label>
					<Input
						id="ci-gw-dhcp"
						bind:value={form.gateway}
						placeholder={$t('vm.cloudInit.gatewayPlaceholder')}
					/>
				</div>
			{/if}

			<div class="grid gap-4 sm:grid-cols-2">
				<div class="space-y-2">
					<Label for="ci-dns">{$t('vm.cloudInit.nameserver')}</Label>
					<Input
						id="ci-dns"
						bind:value={form.nameserver}
						placeholder={$t('vm.cloudInit.nameserverPlaceholder')}
					/>
				</div>
				<div class="space-y-2">
					<Label for="ci-search">{$t('vm.cloudInit.searchdomain')}</Label>
					<Input
						id="ci-search"
						bind:value={form.searchdomain}
						placeholder={$t('vm.cloudInit.searchdomainPlaceholder')}
					/>
				</div>
			</div>

			<div class="flex items-center justify-end gap-2 pt-2">
				<Button
					variant="ghost"
					size="sm"
					disabled={saving || !isDirty}
					onclick={reset}
				>
					{$t('common.cancel')}
				</Button>
				<Button size="sm" disabled={!canSave} onclick={save}>
					{saving ? $t('common.saving') : $t('common.save')}
				</Button>
			</div>
		</div>

		{#if hasCloudInit}
			<Separator class="my-4" />
		{/if}

		<!-- Custom cloud-config YAML (cicustom snippet) -->
		<div class="space-y-3 p-4 pt-2">
			<div class="flex items-center gap-2">
				<FileCode class="h-4 w-4 text-muted-foreground" />
				<span class="text-sm font-medium">{$t('vm.cloudInit.customYaml')}</span>
			</div>

			{#if customYamlLoading}
				<p class="text-xs text-muted-foreground">{$t('common.loading')}</p>
			{:else if customYamlLoadError}
				<p class="text-xs text-destructive">{customYamlLoadError}</p>
				<Button size="sm" variant="ghost" onclick={loadCustomYaml}>
					{$t('common.retry')}
				</Button>
			{:else if !customYamlEditable}
				<!-- SFTP not configured: show a read-only render of the effective
				     cloud-config (Proxmox cloudinit dump). Editing needs SFTP. -->
				<p class="rounded-md border border-yellow-500/30 bg-yellow-500/5 px-3 py-2 text-xs text-yellow-700 dark:text-yellow-400">
					{$t('vm.cloudInit.customYamlReadOnly')}
				</p>
				<Textarea
					value={customYaml}
					rows={12}
					readonly
					class="font-mono text-xs opacity-70"
				/>
			{:else}
				<p class="text-xs text-muted-foreground">
					{$t('vm.cloudInit.customYamlHint')}
				</p>
				<Textarea
					bind:value={customYaml}
					rows={12}
					class="font-mono text-xs"
					placeholder="#cloud-config&#10;package_update: true&#10;packages:&#10;  - curl"
				/>
				<div class="flex items-center justify-end gap-2 pt-1">
					<Button
						variant="ghost"
						size="sm"
						disabled={customYamlSaving || !customYamlDirty}
						onclick={() => (customYaml = customYamlOriginal)}
					>
						{$t('common.cancel')}
					</Button>
					<Button
						size="sm"
						disabled={customYamlSaving || !customYamlDirty}
						onclick={saveCustomYaml}
					>
						{customYamlSaving ? $t('common.saving') : $t('common.save')}
					</Button>
				</div>
			{/if}
		</div>
	{/if}
</div>
