<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import { api, ApiError, type LessonView } from '$lib/api';
	import { session } from '$lib/session.svelte';

	const skill = $derived(page.params.skill ?? '');

	let lesson = $state<LessonView | null>(null);
	let error = $state<string | null>(null);
	let saving = $state(false);
	// Las explicaciones en castellano se abren a pedido, como en los ejercicios.
	let openEs = $state<Record<number, boolean>>({});

	$effect(() => {
		if (!session.ready) return;
		if (!session.user) {
			goto('/');
			return;
		}
		load(skill);
	});

	async function load(s: string) {
		error = null;
		try {
			lesson = await api.lesson(s);
		} catch (err) {
			error = err instanceof ApiError ? `${err.status} · ${err.message}` : 'No se pudo cargar la lección.';
		}
	}

	async function finish() {
		saving = true;
		try {
			await api.completeLesson(skill);
			await goto('/session?block=practice');
		} catch (err) {
			error = err instanceof ApiError ? `${err.status} · ${err.message}` : 'No se pudo guardar.';
		} finally {
			saving = false;
		}
	}

	const kindLabel: Record<string, string> = {
		idea: 'The idea',
		form: 'The form',
		contrast: 'Compare',
		trap: 'Watch out',
		chunks: 'Use it tomorrow'
	};
</script>

<header class="cabecera tapa">
	<a class="etiqueta volver" href="/">← Tablero</a>
	{#if lesson}
		<p class="etiqueta num">{lesson.minutes} min</p>
	{/if}
</header>

<main class="leccion tapa">
	{#if error}
		<p class="error" role="alert">{error}</p>
	{:else if !lesson}
		<p class="etiqueta">Loading</p>
	{:else}
		<p class="etiqueta">Lesson · {lesson.skillEn}</p>
		<h1 class="titular">{lesson.title_en}</h1>
		<p class="meta lectura">{lesson.goal_en}</p>

		{#each lesson.blocks as block, i (block.title_en)}
			<section class="bloque">
				<p class="etiqueta">{kindLabel[block.kind] ?? block.kind}</p>
				<h2 class="sub">{block.title_en}</h2>
				<p class="cuerpo lectura" lang="en">{block.body_en}</p>

				{#if block.kind === 'contrast' && block.pairs}
					<ul class="pares">
						{#each block.pairs as pair (pair.a)}
							<li>
								<p class="oracion" lang="en">{pair.a}</p>
								<p class="oracion" lang="en">{pair.b}</p>
								<p class="diferencia">{pair.difference_es}</p>
							</li>
						{/each}
					</ul>
				{:else if block.examples?.length}
					<ul class="ejemplos">
						{#each block.examples as ex (ex)}
							<li lang="en">{ex}</li>
						{/each}
					</ul>
				{/if}

				{#if block.body_es}
					<button
						class="etiqueta explicar"
						type="button"
						aria-expanded={openEs[i] ?? false}
						onclick={() => (openEs = { ...openEs, [i]: !openEs[i] })}
					>
						{openEs[i] ? 'Ocultar' : 'Explicámelo en castellano'}
					</button>
					{#if openEs[i]}
						<p class="castellano lectura" lang="es">{block.body_es}</p>
					{/if}
				{/if}
			</section>
		{/each}

		<button class="primario" type="button" onclick={finish} disabled={saving}>
			{saving ? 'Saving…' : "I've read it — practice now"}
		</button>
	{/if}
</main>

<style>
	.cabecera {
		display: flex;
		justify-content: space-between;
		align-items: center;
		min-height: 44px;
		border-bottom: 1px solid var(--line);
	}

	.volver {
		text-decoration: none;
		display: inline-flex;
		align-items: center;
		min-height: 44px;
	}

	.leccion {
		display: grid;
		gap: 28px;
		max-width: 760px;
		padding-block: calc(var(--module) * 1.5) calc(var(--module) * 3);
	}

	p {
		margin: 0;
	}

	.titular {
		font-size: clamp(40px, 8vw, 56px);
	}

	.meta {
		font-size: 20px;
		font-weight: 500;
		color: var(--text-muted);
	}

	.bloque {
		display: grid;
		gap: 12px;
		padding-top: 20px;
		border-top: 1px solid var(--line);
	}

	.sub {
		margin: 0;
		font-size: 24px;
		font-weight: 700;
		letter-spacing: -0.02em;
	}

	.cuerpo {
		font-size: 18px;
	}

	.pares {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 20px;
	}

	.pares li {
		display: grid;
		gap: 4px;
		padding-left: 16px;
		border-left: 3px solid var(--baranda);
	}

	.oracion {
		font-family: var(--font-mono);
		font-size: 16px;
		line-height: 1.6;
	}

	.diferencia {
		margin-top: 4px;
		color: var(--text-muted);
	}

	.ejemplos {
		list-style: none;
		margin: 0;
		padding: 0;
		display: grid;
		gap: 10px;
		font-family: var(--font-mono);
		font-size: 16px;
		line-height: 1.6;
	}

	.ejemplos li {
		padding-left: 16px;
		border-left: 1px solid var(--line);
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
		padding: 16px;
		background: var(--surface);
	}

	.primario {
		justify-self: start;
		min-height: 56px;
		padding: 0 28px;
		background: var(--text);
		color: var(--bg);
		border: 0;
		font-weight: 600;
		cursor: pointer;
	}

	.primario:disabled {
		opacity: 0.5;
		cursor: default;
	}

	.error {
		color: var(--oxido);
	}
</style>
