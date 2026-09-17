<script lang="ts">
	import type { Missed } from '$lib/api';

	type Props = { missed: Missed[] };
	let { missed }: Props = $props();

	let open = $state<string | null>(null);

	const toggle = (id: string) => (open = open === id ? null : id);
</script>

<section class="errores">
	<h3 class="etiqueta">What you missed · {missed.length}</h3>
	<p class="hint">These are the ones to look at. Same order you answered them.</p>

	<ul>
		{#each missed as m (m.itemId)}
			<li>
				<p class="etiqueta skill">{m.skillName}</p>
				<p class="oracion" lang="en">{m.question ? `${m.text} — ${m.question}` : m.text}</p>
				<p class="respuesta">
					<span class="etiqueta">Answer</span>
					<span lang="en">{m.expected}</span>
				</p>
				<p class="regla" lang="en">{m.rule}</p>
				<button class="etiqueta explicar" type="button" aria-expanded={open === m.itemId} onclick={() => toggle(m.itemId)}>
					{open === m.itemId ? 'Ocultar' : 'Explicámelo en castellano'}
				</button>
				{#if open === m.itemId}
					<div class="castellano" lang="es">
						<div>
							<p class="etiqueta">La regla</p>
							<p>{m.explainEs.rule}</p>
						</div>
						<div>
							<p class="etiqueta">Una analogía</p>
							<p>{m.explainEs.analogy}</p>
						</div>
						<div>
							<p class="etiqueta">Por qué</p>
							<p>{m.explainEs.why}</p>
						</div>
					</div>
				{/if}
			</li>
		{/each}
	</ul>
</section>

<style>
	.errores {
		display: grid;
		gap: 8px;
	}

	p {
		margin: 0;
	}

	.hint {
		color: var(--text-muted);
	}

	ul {
		list-style: none;
		margin: 8px 0 0;
		padding: 0;
		border-top: 1px solid var(--line);
		max-width: 640px;
	}

	li {
		display: grid;
		gap: 8px;
		padding: 20px 0;
		border-bottom: 1px solid var(--line-soft);
	}

	.skill {
		color: var(--oxido);
	}

	.oracion {
		font-family: var(--font-mono);
		line-height: 1.7;
	}

	.respuesta {
		display: flex;
		flex-wrap: wrap;
		align-items: baseline;
		gap: 12px;
	}

	.respuesta span[lang] {
		font-family: var(--font-mono);
		text-decoration: underline 2px var(--baranda);
		text-underline-offset: 5px;
	}

	.regla {
		padding-left: 16px;
		border-left: 3px solid var(--baranda);
		color: var(--text-muted);
	}

	.explicar {
		justify-self: start;
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
	}

	.castellano > div {
		display: grid;
		gap: 4px;
	}
</style>
