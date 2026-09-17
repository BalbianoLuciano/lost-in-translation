<script lang="ts">
	import '$lib/styles/base.css';
	import favicon from '$lib/assets/favicon.svg';
	import { onMount } from 'svelte';
	import GlossaryButton from '$lib/components/GlossaryButton.svelte';
	import GlossaryPanel from '$lib/components/GlossaryPanel.svelte';
	import { session } from '$lib/session.svelte';
	import { readStoredPref, watchSystemTheme } from '$lib/theme';

	let { children } = $props();

	onMount(() => {
		session.init();
		return watchSystemTheme(readStoredPref);
	});
</script>

<svelte:head>
	<link rel="icon" href={favicon} type="image/svg+xml" />
</svelte:head>

<div class="obra tensores">
	{@render children()}
</div>

{#if session.user}
	<GlossaryButton />
	<GlossaryPanel />
{/if}

<style>
	.obra {
		min-height: 100dvh;
		padding: calc(var(--module) + env(safe-area-inset-top, 0px)) var(--gutter)
			calc(var(--module) + env(safe-area-inset-bottom, 0px));
		display: flex;
		flex-direction: column;
	}
</style>
