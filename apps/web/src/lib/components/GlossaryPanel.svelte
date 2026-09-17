<script lang="ts">
	import type { Cheatsheet } from '$lib/api';
	import { glossary } from '$lib/glossary.svelte';

	let query = $state('');
	let sheet = $state<Cheatsheet | null>(null);
	let input = $state<HTMLInputElement | null>(null);

	const hits = $derived(glossary.search(query));
	const data = $derived(glossary.data);

	$effect(() => {
		if (glossary.open) queueMicrotask(() => input?.focus());
		else {
			query = '';
			sheet = null;
		}
	});

	function onkeydown(e: KeyboardEvent) {
		if (e.key === 'Escape' && glossary.open) {
			if (sheet) sheet = null;
			else glossary.hide();
		}
	}

	const typeLabel = {
		term: 'term',
		chunk: 'chunk',
		phrasal: 'phrasal verb',
		false_friend: 'false friend'
	} as const;
</script>

<svelte:window {onkeydown} />

{#if glossary.open}
	<!-- El fondo cierra el panel; el botón de abajo es el control accesible -->
	<div class="fondo" onclick={() => glossary.hide()} aria-hidden="true"></div>

	<div class="panel" role="dialog" aria-modal="true" aria-label="Glossary">
		<header>
			<p class="etiqueta">Glosario · consulta rápida</p>
			<button class="cerrar etiqueta" type="button" onclick={() => glossary.hide()}>Cerrar</button>
		</header>

		{#if sheet}
			<button class="volver etiqueta" type="button" onclick={() => (sheet = null)}>← Glosario</button>
			<h2 class="titulo">{sheet.title_en}</h2>
			<p class="resumen">{sheet.summary_es}</p>
			<table>
				<tbody>
					{#each sheet.rows as row (row.name)}
						<tr>
							<td>
								<p class="fila-nombre">{row.name}</p>
								<p class="forma" lang="en">{row.form}</p>
								<p class="uso">{row.use_es}</p>
								<p class="ejemplo" lang="en">{row.example}</p>
							</td>
						</tr>
					{/each}
				</tbody>
			</table>
			{#if sheet.notes_es?.length}
				<ul class="notas">
					{#each sheet.notes_es as note (note)}
						<li>{note}</li>
					{/each}
				</ul>
			{/if}
		{:else}
			<label class="buscador">
				<span class="visually-hidden">Buscar</span>
				<input
					bind:this={input}
					bind:value={query}
					type="search"
					placeholder="break, romper, roll out, condicional…"
					autocomplete="off"
					autocapitalize="off"
					spellcheck="false"
				/>
			</label>

			{#if glossary.error}
				<p class="aviso">{glossary.error}</p>
			{:else if !data && glossary.loading}
				<p class="etiqueta">Cargando</p>
			{:else if query.length < 2}
				<p class="etiqueta seccion">Chuletas</p>
				<ul class="lista">
					{#each data?.cheatsheets ?? [] as c (c.id)}
						<li>
							<button type="button" class="fila" onclick={() => (sheet = c)}>
								<span class="clave" lang="en">{c.title_en}</span>
								<span class="valor">{c.title_es}</span>
							</button>
						</li>
					{/each}
				</ul>
				<p class="etiqueta pie">
					{data?.verbs?.length ?? 0} verbos irregulares · {data?.rules?.length ?? 0} reglas de escritura ·
					{data?.terms?.length ?? 0} términos
				</p>
			{:else if hits.length === 0}
				<p class="aviso">Nada con “{query}”. En F2 vas a poder preguntarle a la IA desde acá.</p>
			{:else}
				<ul class="lista">
					{#each hits as hit (hit.kind + (hit.kind === 'verb' ? hit.verb.base : hit.kind === 'term' ? hit.term.term : hit.kind === 'rule' ? hit.rule.id : hit.sheet.id))}
						<li>
							{#if hit.kind === 'verb'}
								<p class="formas" lang="en">
									<strong>{hit.verb.base}</strong> · {hit.verb.past} · {hit.verb.participle}
								</p>
								<p class="valor">{hit.verb.es}</p>
								<p class="ejemplo" lang="en">{hit.verb.example}</p>
								{#if hit.verb.note_es}<p class="nota">{hit.verb.note_es}</p>{/if}
							{:else if hit.kind === 'term'}
								<p class="formas" lang="en">
									<strong>{hit.term.term}</strong>
									<span class="etiqueta tipo">{typeLabel[hit.term.type]}</span>
								</p>
								<p class="valor">{hit.term.es}</p>
								<p class="ejemplo" lang="en">{hit.term.example}</p>
								{#if hit.term.note_es}<p class="nota">{hit.term.note_es}</p>{/if}
							{:else if hit.kind === 'rule'}
								<p class="formas" lang="en"><strong>{hit.rule.title_en}</strong></p>
								<p class="valor">{hit.rule.when_es}</p>
								<p class="ejemplo" lang="en">{hit.rule.examples.join(' · ')}</p>
								{#if hit.rule.note_es}<p class="nota">{hit.rule.note_es}</p>{/if}
							{:else}
								<button type="button" class="fila" onclick={() => (sheet = hit.sheet)}>
									<span class="clave" lang="en">{hit.sheet.title_en}</span>
									<span class="valor">{hit.sheet.summary_es}</span>
								</button>
							{/if}
						</li>
					{/each}
				</ul>
			{/if}
		{/if}
	</div>
{/if}

<style>
	.fondo {
		position: fixed;
		inset: 0;
		background: color-mix(in srgb, var(--bg) 70%, transparent);
		z-index: 20;
	}

	.panel {
		position: fixed;
		z-index: 21;
		inset: auto 0 0 0;
		max-height: 82dvh;
		overflow-y: auto;
		overscroll-behavior: contain;
		background: var(--surface);
		border-top: 1px solid var(--line);
		padding: var(--module) var(--gutter) calc(var(--module) + env(safe-area-inset-bottom, 0px));
		display: grid;
		gap: 12px;
		align-content: start;
	}

	@media (min-width: 900px) {
		.panel {
			inset: 0 0 0 auto;
			width: min(520px, 100%);
			max-height: none;
			border-top: 0;
			border-left: 1px solid var(--line);
			padding-top: calc(var(--module) + env(safe-area-inset-top, 0px));
		}
	}

	header {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
	}

	p {
		margin: 0;
	}

	.cerrar,
	.volver {
		min-height: 44px;
		background: none;
		border: 0;
		padding: 0;
		cursor: pointer;
		color: var(--text);
	}

	.volver {
		justify-self: start;
	}

	input {
		width: 100%;
		min-height: 52px;
		font: inherit;
		font-size: 17px;
		color: var(--text);
		background: var(--bg);
		border: 1px solid var(--line);
		border-radius: 0;
		padding: 0 14px;
	}

	input:focus {
		outline: none;
		border-color: var(--baranda);
	}

	.lista {
		list-style: none;
		margin: 0;
		padding: 0;
		border-top: 1px solid var(--line);
	}

	.lista li {
		display: grid;
		gap: 4px;
		padding: 14px 0;
		border-bottom: 1px solid var(--line-soft);
	}

	.fila {
		width: 100%;
		display: grid;
		gap: 4px;
		text-align: left;
		background: none;
		border: 0;
		padding: 0;
		min-height: 44px;
		cursor: pointer;
		color: inherit;
	}

	.clave,
	.formas {
		font-family: var(--font-mono);
		font-size: 16px;
	}

	.formas strong {
		font-weight: 500;
		color: var(--baranda);
	}

	.tipo {
		margin-left: 8px;
	}

	.valor {
		color: var(--text);
	}

	.ejemplo {
		font-family: var(--font-mono);
		font-size: 14px;
		color: var(--text-muted);
	}

	.nota {
		padding-left: 12px;
		border-left: 2px solid var(--oxido);
		color: var(--text-muted);
	}

	.seccion,
	.pie {
		margin-top: 8px;
	}

	.aviso {
		color: var(--oxido);
	}

	.titulo {
		margin: 0;
		font-size: 32px;
		font-weight: 800;
		letter-spacing: -0.03em;
	}

	.resumen {
		color: var(--text);
		max-width: var(--reading-width);
	}

	table {
		width: 100%;
		border-collapse: collapse;
	}

	td {
		padding: 14px 0;
		border-bottom: 1px solid var(--line-soft);
		display: grid;
		gap: 4px;
	}

	.fila-nombre {
		font-weight: 600;
	}

	.forma {
		font-family: var(--font-mono);
		color: var(--baranda);
	}

	.uso {
		color: var(--text);
	}

	.notas {
		margin: 8px 0 0;
		padding-left: 18px;
		display: grid;
		gap: 8px;
		color: var(--text-muted);
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
