<script lang="ts">
	import { goto } from '$app/navigation';
	import { page } from '$app/state';
	import {
		api,
		ApiError,
		type ItemResponse,
		type NextItem,
		type SessionAnswer,
		type SessionBlock,
		type SessionState
	} from '$lib/api';
	import Exercise from '$lib/components/exercise/Exercise.svelte';
	import Feedback from '$lib/components/exercise/Feedback.svelte';
	import { glossary } from '$lib/glossary.svelte';
	import { session } from '$lib/session.svelte';

	const block = $derived<SessionBlock>(page.url.searchParams.get('block') === 'review' ? 'review' : 'practice');

	let today = $state<SessionState | null>(null);
	let current = $state<NextItem | null>(null);
	let result = $state<SessionAnswer | null>(null);
	let submitting = $state(false);
	let error = $state<string | null>(null);
	let loading = $state(true);
	let shownAt = 0;

	// Mientras hay un ejercicio sin responder, abrir el glosario marca el intento.
	$effect(() => {
		glossary.exerciseActive = Boolean(current) && !result;
		glossary.currentItemId = current?.item.id ?? null;
		return () => {
			glossary.exerciseActive = false;
			glossary.currentItemId = null;
		};
	});

	$effect(() => {
		if (!session.ready) return;
		if (!session.user) {
			goto('/');
			return;
		}
		load(block);
	});

	async function load(b: SessionBlock) {
		loading = true;
		error = null;
		result = null;
		try {
			const [session, next] = await Promise.all([api.session(), api.sessionNext(b)]);
			today = session;
			current = next.next;
			shownAt = performance.now();
		} catch (err) {
			error = message(err);
		} finally {
			loading = false;
		}
	}

	async function answer(response: ItemResponse) {
		if (!current) return;
		submitting = true;
		error = null;
		try {
			result = await api.sessionAnswer({
				block,
				itemId: current.item.id,
				response,
				latencyMs: Math.round(performance.now() - shownAt),
				consulted: glossary.consultedNow
			});
			today = result.state;
		} catch (err) {
			error = message(err);
		} finally {
			submitting = false;
		}
	}

	function next() {
		if (!result) return;
		current = result.next;
		result = null;
		glossary.resetConsulted();
		shownAt = performance.now();
		window.scrollTo({ top: 0 });
	}

	function message(err: unknown): string {
		if (err instanceof ApiError) return `${err.status} · ${err.message}`;
		return 'No se pudo contactar la API.';
	}

	const title = $derived(block === 'review' ? 'Review' : 'Practice');
</script>

<header class="cabecera tapa">
	<a class="etiqueta volver" href="/">← Tablero</a>
	{#if today}
		<p class="etiqueta num">
			{title}
			{#if current}· {current.left + 1} left{/if}
			· Colada {today.coladaToday}
		</p>
	{/if}
</header>

<main class="sesion tapa">
	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}

	{#if loading}
		<p class="etiqueta">Loading</p>
	{:else if current}
		<p class="etiqueta">{current.skillEn}</p>
		{#key current.item.id}
			<Exercise item={current.item} {result} {submitting} onsubmit={answer} />
		{/key}
		{#if result}
			<Feedback {result} consulted={glossary.consultedNow} onnext={next} last={result.next === null} />
		{/if}
	{:else}
		<!-- El bloque terminó por hoy -->
		<p class="etiqueta">{title} · done</p>
		<h1 class="titular">
			{#if block === 'review'}
				Nothing left to review.
			{:else if today?.practice?.locked}
				Read the lesson first.
			{:else}
				That's the practice for today.
			{/if}
		</h1>

		{#if today}
			<p class="bajada lectura">
				Colada {today.coladaToday} today · {today.coladaTotal} in total · Jornal {today.jornal}.
			</p>

			<ul class="siguientes">
				{#if today.review.due > 0 && block !== 'review'}
					<li><a href="/session?block=review">Review {today.review.due} due</a></li>
				{/if}
				{#if today.lesson && today.lesson.status !== 'done'}
					<li><a href="/lesson/{today.lesson.skill}">Lesson · {today.lesson.titleEn}</a></li>
				{/if}
				{#if today.practice && !today.practice.locked && today.practice.done < today.practice.total && block !== 'practice'}
					<li><a href="/session?block=practice">Practice · {today.practice.skillEn}</a></li>
				{/if}
				<li><a href="/">Back to the board</a></li>
			</ul>
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

	.sesion {
		display: grid;
		gap: 24px;
		max-width: 760px;
		padding-block: calc(var(--module) * 1.5) calc(var(--module) * 2);
	}

	p {
		margin: 0;
	}

	.titular {
		font-size: clamp(36px, 7vw, 52px);
	}

	.bajada {
		font-size: 18px;
		color: var(--text-muted);
	}

	.siguientes {
		list-style: none;
		margin: 0;
		padding: 0;
		border-top: 1px solid var(--line);
		max-width: 520px;
	}

	.siguientes li {
		border-bottom: 1px solid var(--line-soft);
	}

	.siguientes a {
		display: block;
		padding: 16px 0;
		min-height: 44px;
		color: inherit;
		text-decoration: none;
		font-weight: 600;
	}

	.siguientes a:hover {
		color: var(--baranda);
	}

	.error {
		color: var(--oxido);
	}
</style>
