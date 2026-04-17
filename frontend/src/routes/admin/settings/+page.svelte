<script lang="ts">
	import { t } from 'svelte-i18n';
	import { toast } from 'svelte-sonner';
	import { DatabaseIcon, UploadIcon, DownloadIcon, ClockIcon } from 'phosphor-svelte';
	import { Button } from '$lib/components/ui/button';
	import ConfirmDialog from '$lib/components/forms/ConfirmDialog.svelte';
	import AuditLog from '$lib/components/admin/AuditLog.svelte';
	import SettingsOverview from '$lib/components/admin/SettingsOverview.svelte';
	import { exportDB, importDB } from '$lib/api/admin/db';

	let exportLoading = $state(false);
	let importLoading = $state(false);
	let importConfirmOpen = $state(false);
	let pendingImportFile = $state<File | null>(null);
	let importFileInput: HTMLInputElement;

	async function handleExport() {
		exportLoading = true;
		try {
			await exportDB();
			toast.success($t('admin.settings.db.exportSuccess'));
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			exportLoading = false;
		}
	}

	function handleImportPick(e: Event) {
		const file = (e.target as HTMLInputElement).files?.[0];
		if (!file) return;
		pendingImportFile = file;
		importConfirmOpen = true;
		(e.target as HTMLInputElement).value = '';
	}

	async function handleImportConfirm() {
		if (!pendingImportFile) return;
		importLoading = true;
		importConfirmOpen = false;
		try {
			await importDB(pendingImportFile);
			toast.success($t('admin.settings.db.importSuccess'));
		} catch (e) {
			toast.error((e as Error).message);
		} finally {
			importLoading = false;
			pendingImportFile = null;
		}
	}
</script>

<svelte:head>
	<title>PVMSS — {$t('admin.settings.title')}</title>
</svelte:head>

<div class="pv-header -mx-6 -mt-6 mb-6">
	<div class="pv-header-flex">
		<div>
			<p class="pv-eyebrow">{$t('nav.administration')}</p>
			<h1 class="pv-title">{$t('admin.settings.title')}</h1>
			<p class="pv-subtitle">{$t('admin.settings.subtitle')}</p>
		</div>
	</div>
</div>

<div class="pv-content-width space-y-8">

	<!-- Configuration overview -->
	<SettingsOverview />

	<!-- Database management -->
	<section>
		<div class="flex items-center gap-2 mb-4">
			<div class="pv-resource-icon" style="width:28px;height:28px;">
				<DatabaseIcon class="h-3.5 w-3.5" />
			</div>
			<h2 class="text-base font-semibold">{$t('admin.settings.db.title')}</h2>
		</div>

		<div class="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
			<!-- Export -->
			<div class="rounded-xl border border-border bg-card p-5 space-y-3">
				<div>
					<p class="font-medium text-sm">{$t('admin.settings.db.exportTitle')}</p>
					<p class="text-xs text-muted-foreground mt-1">{$t('admin.settings.db.exportDesc')}</p>
				</div>
				<Button
					variant="outline"
					size="sm"
					class="w-full"
					disabled={exportLoading}
					onclick={handleExport}
				>
					<DownloadIcon class="h-4 w-4 mr-2" />
					{exportLoading ? $t('common.loading') : $t('admin.settings.db.exportBtn')}
				</Button>
			</div>

			<!-- Import -->
			<div class="rounded-xl border border-border bg-card p-5 space-y-3">
				<div>
					<p class="font-medium text-sm">{$t('admin.settings.db.importTitle')}</p>
					<p class="text-xs text-muted-foreground mt-1">{$t('admin.settings.db.importDesc')}</p>
				</div>
				<input
					bind:this={importFileInput}
					type="file"
					accept=".db"
					class="hidden"
					onchange={handleImportPick}
				/>
				<Button
					variant="outline"
					size="sm"
					class="w-full"
					disabled={importLoading}
					onclick={() => importFileInput.click()}
				>
					<UploadIcon class="h-4 w-4 mr-2" />
					{importLoading ? $t('common.loading') : $t('admin.settings.db.importBtn')}
				</Button>
			</div>
		</div>
	</section>

	<!-- Audit log -->
	<section>
		<div class="flex items-center gap-2 mb-4">
			<div class="pv-resource-icon" style="width:28px;height:28px;">
				<ClockIcon class="h-3.5 w-3.5" />
			</div>
			<h2 class="text-base font-semibold">{$t('admin.settings.audit.title')}</h2>
		</div>
		<AuditLog />
	</section>

</div>

<!-- Import confirmation dialog -->
<ConfirmDialog
	open={importConfirmOpen}
	title={$t('admin.settings.db.importConfirmTitle')}
	description={$t('admin.settings.db.importConfirmDesc')}
	confirmLabel={$t('admin.settings.db.importBtn')}
	variant="destructive"
	onConfirm={handleImportConfirm}
	onCancel={() => { importConfirmOpen = false; pendingImportFile = null; }}
/>
