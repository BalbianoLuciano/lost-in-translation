<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError, type Distincion } from '$lib/api';
	import { session } from '$lib/session.svelte';

	let todas = $state<Distincion[]>([]);
	let error = $state<string | null>(null);
	let loading = $state(true);

	$effect(() => {
		if (!session.ready) return;
		if (!session.user) {
			goto('/');
			return;
		}
		load();
	});

	async function load() {
		try {
			todas = (await api.achievements()).achievements;
		} catch (err) {
			error =
				err instanceof ApiError
					? `${err.status} · ${err.message}`
					: 'No se pudieron cargar las distinciones.';
		} finally {
			loading = false;
		}
	}

	type Grupo = { obra: number; obraName: string; piezas: Distincion[]; entera: Distincion | null };

	// El servidor ya las manda en orden de currículum: las piezas de cada obra y
	// después la obra. Acá sólo se agrupan, sin reordenar nada.
	const grupos = $derived(
		todas.reduce<Grupo[]>((acc, d) => {
			let g = acc.at(-1);
			if (!g || g.obra !== d.obra) {
				g = { obra: d.obra, obraName: d.obraName, piezas: [], entera: null };
				acc.push(g);
			}
			if (d.kind === 'obra') g.entera = d;
			else g.piezas.push(d);
			return acc;
		}, [])
	);

	const ganadas = $derived(todas.filter((d) => d.earned).length);

	const formato = new Intl.DateTimeFormat('en-GB', {
		day: '2-digit',
		month: 'short',
		year: 'numeric'
	});
	const cuando = (iso: string | null) => (iso ? formato.format(new Date(iso)) : '');
</script>

<header class="cabecera tapa">
	<a class="etiqueta volver" href="/map">← Tu obra</a>
	<p class="etiqueta num">{ganadas} / {todas.length} distinciones</p>
</header>

<main class="distinciones tapa">
	{#if error}
		<p class="error" role="alert">{error}</p>
	{:else if loading}
		<p class="etiqueta">Loading</p>
	{:else}
		<h1 class="titular">Distinciones.</h1>
		<p class="bajada lectura">
			One for every topic you set in place, one for every obra you finish. A piece can rust later
			and the distinction stays: it records that you got there, not that you're still there.
		</p>

		{#each grupos as grupo (grupo.obra)}
			<section class="obra">
				<header class="rotulo">
					<p class="etiqueta">Obra {grupo.obra} · {grupo.obraName}</p>
					{#if grupo.piezas.length > 0}
						<p class="etiqueta num">
							{grupo.piezas.filter((p) => p.earned).length} / {grupo.piezas.length}
						</p>
					{/if}
				</header>

				{#if grupo.piezas.length === 0}
					<p class="vacia">No pieces drawn yet — this obra is still on paper.</p>
				{:else}
					<ul class="piezas">
						{#each grupo.piezas as pieza (pieza.code)}
							<li class="pieza" class:ganada={pieza.earned}>
								<span class="nombre">{pieza.nameEn}</span>
								<span class="es">{pieza.nameEs}</span>
								<span class="fecha num">{pieza.earned ? cuando(pieza.earnedAt) : 'Not yet'}</span>
							</li>
						{/each}
					</ul>
				{/if}

				{#if grupo.entera}
					<div class="entera" class:ganada={grupo.entera.earned}>
						<span class="etiqueta">The whole obra</span>
						<span class="nombre">{grupo.entera.nameEn}</span>
						<span class="fecha num">
							{grupo.entera.earned ? cuando(grupo.entera.earnedAt) : 'Every piece has to set'}
						</span>
					</div>
				{/if}
			</section>
		{/each}
	{/if}
</main>

<style>
	.cabecera {
		display: flex;
		justify-content: space-between;
		align-items: center;
		gap: 12px;
		min-height: 44px;
		border-bottom: 1px solid var(--line);
	}

	.volver {
		text-decoration: none;
		display: inline-flex;
		align-items: center;
		min-height: 44px;
	}

	.distinciones {
		display: grid;
		gap: 24px;
		padding-block: calc(var(--module) * 1.5) calc(var(--module) * 3);
	}

	p {
		margin: 0;
	}

	.titular {
		font-size: clamp(40px, 8vw, 56px);
	}

	.bajada {
		font-size: 18px;
		color: var(--text-muted);
	}

	.obra {
		display: grid;
		gap: 10px;
		padding-top: var(--module);
	}

	.rotulo {
		display: flex;
		justify-content: space-between;
		align-items: baseline;
		gap: 12px;
		border-bottom: 1px solid var(--line-soft);
		padding-bottom: 6px;
	}

	.vacia {
		color: var(--text-muted);
		font-size: 15px;
	}

	.piezas {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 10px;
		grid-template-columns: repeat(auto-fill, minmax(230px, 1fr));
	}

	/* El mismo lenguaje del pilar: lo que falta va en línea punteada y lo ganado
	   apoya en hormigón. */
	.pieza,
	.entera {
		display: grid;
		gap: 2px;
		padding: 12px 14px;
		border: 1.25px dashed var(--line);
		color: var(--text-muted);
	}

	.pieza.ganada,
	.entera.ganada {
		border-style: solid;
		background: var(--hormigon-luz);
		color: var(--vano);
	}

	.entera {
		margin-top: 4px;
		border-width: 2px;
	}

	.entera.ganada {
		border-color: var(--baranda);
	}

	/* Sobre el hormigón el rótulo tiene que leerse igual que el resto */
	.entera.ganada .etiqueta {
		color: var(--vano);
		opacity: 0.7;
	}

	.nombre {
		font-weight: 600;
		line-height: 1.3;
	}

	.es {
		font-size: 14px;
		opacity: 0.8;
	}

	.fecha {
		font-family: var(--font-mono);
		font-size: 12px;
		letter-spacing: 0.06em;
		text-transform: uppercase;
		opacity: 0.75;
	}

	.error {
		color: var(--oxido);
	}
</style>
