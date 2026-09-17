<script lang="ts">
	import { glossary } from '$lib/glossary.svelte';
</script>

{#if !glossary.open}
	<button class="flotante" type="button" onclick={() => glossary.show()}>
		<span class="signo" aria-hidden="true">?</span>
		<span class="visually-hidden">Abrir el glosario</span>
	</button>
{/if}

<style>
	.flotante {
		position: fixed;
		right: var(--gutter);
		bottom: calc(var(--module) + env(safe-area-inset-bottom, 0px));
		z-index: 19;
		width: 56px;
		height: 56px;
		display: grid;
		place-items: center;
		background: var(--text);
		color: var(--bg);
		border: 0;
		cursor: pointer;
		box-shadow: 4px 4px 0 0 var(--line);
	}

	.flotante:hover {
		background: var(--baranda);
	}

	.signo {
		font-family: var(--font-mono);
		font-size: 22px;
		font-weight: 500;
		line-height: 1;
	}

	.visually-hidden {
		position: absolute;
		width: 1px;
		height: 1px;
		overflow: hidden;
		clip-path: inset(50%);
		white-space: nowrap;
	}
</style>
