<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError, type MapObra, type MapSkill, type SkillState } from '$lib/api';
	import Pilar from '$lib/components/Pilar.svelte';
	import type { Piece } from '$lib/pilar';
	import { session } from '$lib/session.svelte';

	let obras = $state<MapObra[]>([]);
	let error = $state<string | null>(null);
	let selected = $state<string | null>(null);

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
			obras = (await api.map()).obras.filter((o) => o.pieces.length > 0);
		} catch (err) {
			error = err instanceof ApiError ? `${err.status} · ${err.message}` : 'No se pudo cargar el mapa.';
		}
	}

	const skills = $derived(obras.flatMap((o) => o.pieces.flatMap((p) => p.skills)));
	const count = (state: SkillState) => skills.filter((s) => s.state === state).length;
	const chosen = $derived(skills.find((s) => s.id === selected) ?? null);

	function piecesOf(obra: MapObra): Piece[] {
		return obra.pieces.flatMap((p) =>
			p.skills.map((s) => ({
				id: s.id,
				name: s.nameEn,
				state: s.state,
				mastery: s.mastery,
				items: s.items
			}))
		);
	}

	const estado: Record<SkillState, string> = {
		calzada: 'Calzada — it sets in place',
		suspendida: 'Suspendida — it still floats',
		oxidada: 'Oxidada — it slipped, review it',
		plano: 'Plano — still on the drawing'
	};
</script>

<header class="cabecera tapa">
	<a class="etiqueta volver" href="/">← Tablero</a>
	<p class="etiqueta num">{count('calzada')} calzadas · {skills.length} temas</p>
</header>

<main class="mapa tapa">
	{#if error}
		<p class="error" role="alert">{error}</p>
	{:else if obras.length === 0}
		<p class="etiqueta">Loading</p>
	{:else}
		<h1 class="titular">Your obra.</h1>
		<p class="bajada lectura">
			One piece per topic. What you don't master yet floats with its joint open; what you master
			settles into place. Tap a piece.
		</p>

		<div class="obras">
			{#each obras as obra (obra.id)}
				<section class="obra">
					<p class="etiqueta">Obra {obra.id} · {obra.name}</p>
					<h2 class="sub">{obra.topicEn}</h2>
					<Pilar pieces={piecesOf(obra)} {selected} onselect={(id) => (selected = id)} width={200} />
				</section>
			{/each}
		</div>

		{#if chosen}
			<aside class="detalle tapa">
				<p class="etiqueta">{estado[chosen.state]}</p>
				<h3 class="sub">{chosen.nameEn}</h3>
				<p class="es">{chosen.nameEs}</p>
				<p class="num datos">
					{Math.round(chosen.mastery * 100)}% · {chosen.items} exercises
				</p>
				{#if chosen.hasLesson}
					<a class="primario" href="/lesson/{chosen.id}">Open the lesson</a>
				{:else}
					<p class="etiqueta">Lesson coming soon</p>
				{/if}
			</aside>
		{/if}
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

	.mapa {
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

	.obras {
		display: grid;
		gap: calc(var(--module) * 2);
		grid-template-columns: repeat(auto-fit, minmax(220px, 1fr));
		align-items: start;
		padding-top: var(--module);
	}

	.obra {
		display: grid;
		gap: 6px;
		align-content: end;
	}

	.sub {
		margin: 0;
		font-size: 22px;
		font-weight: 700;
		letter-spacing: -0.02em;
	}

	.detalle {
		position: sticky;
		bottom: 0;
		display: grid;
		gap: 8px;
		justify-items: start;
		padding: 20px 0;
		border-top: 1px solid var(--line);
		background: var(--bg);
	}

	.es {
		color: var(--text-muted);
	}

	.datos {
		font-family: var(--font-mono);
		font-size: 13px;
		color: var(--text-muted);
	}

	.primario {
		display: inline-flex;
		align-items: center;
		min-height: 48px;
		padding: 0 24px;
		background: var(--text);
		color: var(--bg);
		font-weight: 600;
		text-decoration: none;
	}

	.error {
		color: var(--oxido);
	}
</style>
