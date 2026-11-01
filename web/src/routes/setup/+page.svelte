<script lang="ts">
	import Alert from '#lib/shared/ui/Alert.svelte';
	import Button from '#lib/shared/ui/Button.svelte';
	import Card from '#lib/shared/ui/Card.svelte';
	import Checkbox from '#lib/shared/ui/Checkbox.svelte';
	import FormField from '#lib/shared/ui/FormField.svelte';
	import TextField from '#lib/shared/ui/TextField.svelte';
	import UserIcon from '#lib/shared/ui/icons/UserIcon.svelte';
	import LockIcon from '#lib/shared/ui/icons/LockIcon.svelte';
	import { m } from '#lib/paraglide/messages.js';
	import { getSessionContext } from '#lib/features/auth/session.svelte.js';
	import { getSetupContext } from '#lib/features/setup/setup.svelte.js';
	import {
		adminLogin,
		createSetupAdmin,
		createSetupCluster,
		clusterInputFromSetup,
		isValidClusterName,
		probeCluster,
		setupStep,
		suggestClusterName,
		translateSetupError,
		type ClusterProbe,
		type SetupClusterInput
	} from '#lib/features/setup/setup.js';

	const session = getSessionContext();
	const setup = getSetupContext();

	const step = $derived(
		setup.status === null ? 'loading' : setupStep(setup.status, session.principal !== null)
	);
	const stepLabel = $derived(
		step === 'admin'
			? m['setup.step.admin']()
			: step === 'signin'
				? m['setup.step.signin']()
				: m['setup.step.cluster']()
	);

	const ERROR_ID = 'setup-error';
	let busy = $state(false);
	let error = $state<string | null>(null);

	let username = $state('');
	let password = $state('');
	let confirm = $state('');

	let signinUsername = $state('');
	let signinPassword = $state('');

	let url = $state('');
	let tokenId = $state('');
	let tokenSecret = $state('');
	let skipTls = $state(false);
	let name = $state('');
	let displayName = $state('');
	let nameTouched = $state(false);
	let displayTouched = $state(false);
	let probing = $state(false);
	let probe = $state<ClusterProbe | null>(null);

	const nameValid = $derived(isValidClusterName(name.trim()));

	function connection(): SetupClusterInput {
		return { url, tokenId, tokenSecret, tlsInsecureSkipVerify: skipTls };
	}

	async function submitAdmin(): Promise<void> {
		if (busy) return;
		error = null;
		if (password !== confirm) {
			error = m['setup.admin.mismatch']();
			return;
		}
		busy = true;
		try {
			await createSetupAdmin({ username: username.trim(), password });
			// Sign straight in; on failure the status refresh lands the user on
			// the sign-in step where they retry with the same credentials.
			try {
				await adminLogin(username.trim(), password);
				await session.load();
			} catch {
				// best effort - the signin step retries
			}
			await setup.load();
		} catch (err: unknown) {
			error = translateSetupError(err);
		} finally {
			busy = false;
		}
	}

	async function submitSignin(): Promise<void> {
		if (busy) return;
		error = null;
		busy = true;
		try {
			await adminLogin(signinUsername, signinPassword);
			await session.load();
			await setup.load();
		} catch (err: unknown) {
			error = translateSetupError(err);
		} finally {
			busy = false;
		}
	}

	async function runProbe(): Promise<void> {
		probing = true;
		probe = null;
		error = null;
		try {
			applyProbe(await probeCluster(connection()));
		} catch (err: unknown) {
			error = translateSetupError(err);
		} finally {
			probing = false;
		}
	}

	// A successful probe reports the cluster's own name: fill the editable
	// display name and derive the immutable identifier from it, without
	// clobbering anything the admin already typed.
	function applyProbe(result: ClusterProbe): void {
		probe = result;
		if (result.status !== 'ok' || !result.clusterName) return;
		if (!displayTouched) displayName = result.clusterName;
		const suggestion = suggestClusterName(result.clusterName);
		if (!nameTouched && suggestion !== '') name = suggestion;
	}

	async function submitCluster(): Promise<void> {
		if (busy || !nameValid) {
			error = nameValid ? error : m['setup.error.invalidClusterName']();
			return;
		}
		error = null;
		busy = true;
		try {
			await createSetupCluster(clusterInputFromSetup(name, displayName, connection()));
			// Status flips to configured: the layout guard navigates to '/'.
			await setup.load();
		} catch (err: unknown) {
			error = translateSetupError(err);
		} finally {
			busy = false;
		}
	}
</script>

<svelte:head>
	<title>{m['setup.title']()}</title>
</svelte:head>

<div class="flex w-full flex-1 flex-col items-center justify-center">
	<div class="w-full max-w-md">
		<Card pad="lg" class="flex flex-col gap-4">
			{#if step === 'loading' || step === 'done'}
				<div>
					<h1 class="text-2xl font-semibold tracking-tight">{m['setup.heading']()}</h1>
					<p class="mt-2 text-sm text-muted-foreground">{m['common.loading']()}</p>
				</div>
			{:else}
				<div>
					<p class="text-xs font-medium uppercase tracking-wide text-muted-foreground-subtle">
						{stepLabel}
					</p>
					<h1 class="mt-1 text-2xl font-semibold tracking-tight">
						{#if step === 'admin'}{m['setup.admin.heading']()}
						{:else if step === 'signin'}{m['setup.signin.heading']()}
						{:else}{m['setup.cluster.heading']()}{/if}
					</h1>
					<p class="mt-2 text-sm text-muted-foreground">
						{#if step === 'admin'}{m['setup.admin.description']()}
						{:else if step === 'signin'}{m['setup.signin.description']()}
						{:else}{m['setup.cluster.description']()}{/if}
					</p>
				</div>

				{#if step === 'admin'}
					<form class="grid gap-4" onsubmit={(event) => { event.preventDefault(); void submitAdmin(); }}>
						<FormField label={m['setup.admin.username']()} hint={m['setup.admin.usernameHint']()} required>
							{#snippet children({ id, describedBy })}
								<TextField {id} describedBy={describedBy} bind:value={username} autocomplete="username" required>
									{#snippet leading()}<UserIcon />{/snippet}
								</TextField>
							{/snippet}
						</FormField>
						<FormField label={m['setup.admin.password']()} hint={m['setup.admin.passwordHint']()} required>
							{#snippet children({ id, describedBy })}
								<TextField {id} describedBy={describedBy} type="password" autocomplete="new-password" bind:value={password} reveal required>
									{#snippet leading()}<LockIcon />{/snippet}
								</TextField>
							{/snippet}
						</FormField>
						<FormField label={m['setup.admin.confirm']()} required>
							{#snippet children({ id, describedBy })}
								<TextField {id} describedBy={describedBy} type="password" autocomplete="new-password" bind:value={confirm} reveal required>
									{#snippet leading()}<LockIcon />{/snippet}
								</TextField>
							{/snippet}
						</FormField>
						{#if error}<Alert id={ERROR_ID} tabindex={-1}>{error}</Alert>{/if}
						<Button type="submit" loading={busy}>
							{busy ? m['setup.admin.creating']() : m['setup.admin.create']()}
						</Button>
					</form>
				{:else if step === 'signin'}
					<form class="grid gap-4" onsubmit={(event) => { event.preventDefault(); void submitSignin(); }}>
						<FormField label={m['login.localUsername']()} hint={m['setup.signin.usernameHint']()}>
							{#snippet children({ id, describedBy })}
								<TextField {id} describedBy={describedBy} bind:value={signinUsername} autocomplete="username" placeholder="admin">
									{#snippet leading()}<UserIcon />{/snippet}
								</TextField>
							{/snippet}
						</FormField>
						<FormField label={m['login.password']()} required>
							{#snippet children({ id, describedBy })}
								<TextField {id} describedBy={describedBy} type="password" autocomplete="current-password" bind:value={signinPassword} reveal required>
									{#snippet leading()}<LockIcon />{/snippet}
								</TextField>
							{/snippet}
						</FormField>
						{#if error}<Alert id={ERROR_ID} tabindex={-1}>{error}</Alert>{/if}
						<Button type="submit" loading={busy}>
							{busy ? m['setup.signin.submitting']() : m['setup.signin.submit']()}
						</Button>
					</form>
				{:else}
					<form class="grid gap-4" onsubmit={(event) => { event.preventDefault(); void submitCluster(); }}>
						<FormField label={m['setup.cluster.url']()} required>
							{#snippet children({ id, describedBy })}
								<TextField {id} describedBy={describedBy} bind:value={url} placeholder="https://pve.example.com:8006/api2/json" required />
							{/snippet}
						</FormField>
						<FormField label={m['setup.cluster.tokenId']()} required>
							{#snippet children({ id, describedBy })}
								<TextField {id} describedBy={describedBy} bind:value={tokenId} placeholder="pvmss-svc@pve!token" required />
							{/snippet}
						</FormField>
						<FormField label={m['setup.cluster.tokenSecret']()} required>
							{#snippet children({ id, describedBy })}
								<TextField {id} describedBy={describedBy} type="password" bind:value={tokenSecret} reveal required>
									{#snippet leading()}<LockIcon />{/snippet}
								</TextField>
							{/snippet}
						</FormField>
						<Checkbox label={m['setup.cluster.skipTls']()} bind:checked={skipTls} />
						<div class="flex gap-2">
							<Button type="button" variant="secondary" onclick={() => void runProbe()} loading={probing}>
								{probing ? m['setup.cluster.testing']() : m['setup.cluster.test']()}
							</Button>
						</div>
						{#if probe}
							{#if probe.status === 'ok'}
								<Alert tone="info" role="status">
									{m['setup.cluster.testOk']({
										name: probe.clusterName ?? '-',
										version: probe.proxmoxVersion ?? '-',
										nodes: probe.nodeCount ?? 0,
										vms: probe.vmCount ?? 0
									})}
								</Alert>
							{:else}
								<Alert tone="warning" role="status">
									{m['setup.cluster.testWarn']()}
									{#if probe.message}<span class="block font-normal">{probe.message}</span>{/if}
								</Alert>
							{/if}
						{/if}
						<FormField
							label={m['setup.cluster.name']()}
							hint={m['setup.cluster.nameHint']()}
							required
						>
							{#snippet children({ id, describedBy })}
								<TextField
									{id}
									describedBy={describedBy}
									bind:value={() => name, (value) => { name = value; nameTouched = true; }}
									required
								/>
							{/snippet}
						</FormField>
						{#if probe?.status === 'ok' && probe.clusterName}
							<p class="-mt-2 text-xs text-muted-foreground">{m['setup.cluster.suggested']({ name: probe.clusterName })}</p>
						{/if}
						<FormField label={m['setup.cluster.displayName']()} hint={m['setup.cluster.displayNameHint']()}>
							{#snippet children({ id, describedBy })}
								<TextField
									{id}
									describedBy={describedBy}
									bind:value={() => displayName, (value) => { displayName = value; displayTouched = true; }}
								/>
							{/snippet}
						</FormField>
						{#if error}<Alert id={ERROR_ID} tabindex={-1}>{error}</Alert>{/if}
						<Button type="submit" loading={busy} disabled={!nameValid}>
							{busy ? m['setup.cluster.creating']() : m['setup.cluster.create']()}
						</Button>
					</form>
				{/if}
			{/if}
		</Card>
	</div>
</div>
