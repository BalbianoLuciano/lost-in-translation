<script lang="ts">
	import type { ThemePref } from '$lib/theme';

	type Props = { value: ThemePref; onchange: (pref: ThemePref) => void };
	let { value, onchange }: Props = $props();

	const options: { pref: ThemePref; label: string }[] = [
		{ pref: 'system', label: 'System' },
		{ pref: 'dark', label: 'Dark' },
		{ pref: 'light', label: 'Light' }
	];
</script>

<div class="toggle" role="radiogroup" aria-label="Theme">
	{#each options as o (o.pref)}
		<button
			type="button"
			role="radio"
			aria-checked={value === o.pref}
			class="etiqueta"
			onclick={() => onchange(o.pref)}
		>
			{o.label}
		</button>
	{/each}
</div>

<style>
	.toggle {
		display: inline-flex;
		border: 1px solid var(--line);
	}
	button {
		min-height: 44px;
		padding: 0 14px;
		background: var(--bg);
		border: 0;
		cursor: pointer;
	}
	button + button {
		border-left: 1px solid var(--line);
	}
	button[aria-checked='true'] {
		background: var(--text);
		color: var(--bg);
	}
</style>
