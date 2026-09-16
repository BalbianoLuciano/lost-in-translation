<script lang="ts">
	import type { AnswerResult } from '$lib/api';

	type Props = { result: AnswerResult; onnext: () => void; last: boolean };
	let { result, onnext, last }: Props = $props();

	let explainOpen = $state(false);
	let nextButton = $state<HTMLButtonElement | null>(null);

	$effect(() => {
		nextButton?.focus();
	});
</script>

<section class="feedback" aria-live="polite">
	<p class="veredicto" class:ok={result.correct} class:mal={!result.correct}>
		{#if result.correct}
			Correct.
		{:else if result.partOk}
			Right word, wrong fix.
		{:else}
			Not quite.
		{/if}
	</p>

	{#if !result.correct}
		<p class="esperada">
			<span class="etiqueta">Answer</span>
			<span lang="en">{result.expected}</span>
		</p>
	{/if}

	<!-- Ley 02: la regla siempre, bien o mal -->
	<p class="regla" lang="en">{result.rule}</p>

	<div class="acciones">
		<button bind:this={nextButton} class="primario" type="button" onclick={onnext}>
			{last ? 'See results' : 'Next'}
		</button>
		<button
			class="explicar etiqueta"
			type="button"
			aria-expanded={explainOpen}
			onclick={() => (explainOpen = !explainOpen)}
		>
			{explainOpen ? 'Ocultar' : 'Explicámelo en castellano'}
		</button>
	</div>

	{#if explainOpen}
		<div class="castellano" lang="es">
			<div>
				<p class="etiqueta">La regla</p>
				<p>{result.explainEs.rule}</p>
			</div>
			<div>
				<p class="etiqueta">Una analogía</p>
				<p>{result.explainEs.analogy}</p>
			</div>
			<div>
				<p class="etiqueta">Por qué</p>
				<p>{result.explainEs.why}</p>
			</div>
		</div>
	{/if}
</section>

<style>
	.feedback {
		display: grid;
		gap: 16px;
		padding-top: 20px;
		border-top: 1px solid var(--line);
	}

	p {
		margin: 0;
	}

	.veredicto {
		font-weight: 800;
		font-size: 26px;
		letter-spacing: -0.02em;
	}

	.veredicto.ok {
		color: var(--baranda);
	}

	.veredicto.mal {
		color: var(--oxido);
	}

	.esperada {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 12px;
	}

	.esperada span[lang] {
		font-family: var(--font-mono);
		font-size: 19px;
		text-decoration: underline 2px var(--baranda);
		text-underline-offset: 5px;
	}

	.regla {
		max-width: var(--reading-width);
		padding-left: 16px;
		border-left: 3px solid var(--baranda);
	}

	.acciones {
		display: flex;
		flex-wrap: wrap;
		align-items: center;
		gap: 12px 24px;
	}

	.primario {
		min-height: 52px;
		padding: 0 28px;
		background: var(--text);
		color: var(--bg);
		border: 0;
		font-weight: 600;
		cursor: pointer;
	}

	.explicar {
		min-height: 44px;
		background: none;
		border: 0;
		border-bottom: 1px solid var(--line);
		padding: 0;
		cursor: pointer;
		color: var(--text);
	}

	.castellano {
		display: grid;
		gap: 16px;
		padding: 20px;
		background: var(--surface);
		max-width: var(--reading-width);
	}

	.castellano > div {
		display: grid;
		gap: 4px;
	}
</style>
