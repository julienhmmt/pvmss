<script lang="ts">
	/**
	 * Textarea — the shared multiline input. Uses .pv-input so it matches the
	 * rest of the form vocabulary. Supports a monospace variant for technical
	 * content (SSH keys, cloud-init), an optional character count, and
	 * auto-grow to fit content. All transitions are guarded by the global
	 * prefers-reduced-motion rule in app.css.
	 */
	interface Props {
		id?: string;
		value: string;
		rows?: number;
		minRows?: number;
		maxRows?: number;
		mono?: boolean;
		placeholder?: string;
		describedBy?: string | undefined;
		invalid?: boolean;
		required?: boolean;
		disabled?: boolean;
		readonly?: boolean;
		maxLength?: number;
		showCount?: boolean;
		autoGrow?: boolean;
		name?: string;
		class?: string;
		/** Called when Cmd/Ctrl+Enter is pressed inside the textarea. */
		onCmdEnter?: () => void;
		[key: string]: unknown;
	}

	let {
		id,
		value = $bindable(''),
		rows = 4,
		minRows = 3,
		maxRows = 12,
		mono = false,
		placeholder,
		describedBy,
		invalid = false,
		required = false,
		disabled = false,
		readonly = false,
		maxLength,
		showCount = false,
		autoGrow = false,
		name,
		class: klass = '',
		onCmdEnter,
		...rest
	}: Props = $props();

	const strValue = $derived(value ?? '');
	const count = $derived(strValue.length);

	function handleKeydown(event: KeyboardEvent): void {
		if (onCmdEnter && (event.metaKey || event.ctrlKey) && event.key === 'Enter') {
			event.preventDefault();
			onCmdEnter();
		}
	}

	/** Svelte action: grow the textarea to fit its content, clamped to maxRows. */
	function autogrow(node: HTMLTextAreaElement, enabled: boolean): { update(e: boolean): void; destroy(): void } {
		let active = enabled;
		const minPx = minRows * 1.25 * 16;
		const maxPx = maxRows * 1.25 * 16;
		const resize = (): void => {
			if (!active) return;
			node.style.height = 'auto';
			node.style.height = `${Math.min(Math.max(node.scrollHeight, minPx), maxPx)}px`;
		};
		resize();
		node.addEventListener('input', resize);
		return {
			update(e: boolean): void {
				active = e;
				resize();
			},
			destroy(): void {
				node.removeEventListener('input', resize);
			}
		};
	}
</script>

<div class="grid gap-1 {klass}">
	<textarea
		{id}
		{name}
		class="pv-input resize-y {mono ? 'font-mono text-xs' : ''}"
		{rows}
		{placeholder}
		{required}
		{disabled}
		{readonly}
		maxlength={maxLength}
		aria-invalid={invalid ? 'true' : undefined}
		aria-describedby={describedBy}
		bind:value
		use:autogrow={autoGrow}
		onkeydown={handleKeydown}
		{...rest}
	></textarea>
	{#if showCount && maxLength}
		<p class="flex justify-end text-xs text-muted-foreground-subtle">{count}/{maxLength}</p>
	{/if}
</div>
