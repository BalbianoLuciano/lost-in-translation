<script lang="ts">
	import { page } from '$app/state';
	import { goto } from '$app/navigation';
	import { api, ApiError, type AnswerResult, type ItemResponse, type PartView, type RunState } from '$lib/api';
	import Exercise from '$lib/components/exercise/Exercise.svelte';
	import Feedback from '$lib/components/exercise/Feedback.svelte';
	import MissedList from '$lib/components/MissedList.svelte';
	import PlacementSummary from '$lib/components/PlacementSummary.svelte';
	import { glossary } from '$lib/glossary.svelte';
	import { session } from '$lib/session.svelte';

	const partId = $derived(page.params.part ?? '');

	let run = $state<RunState | null>(null);
	let doneView = $state<PartView | null>(null);
	let result = $state<AnswerResult | null>(null);
	let submitting = $state(false);
	let error = $state<string | null>(null);
	let shownAt = 0;

	// Mientras hay un ejercicio sin responder, abrir el glosario marca el intento.
	$effect(() => {
		glossary.exerciseActive = Boolean(run?.next) && !result;
		glossary.currentItemId = run?.next?.id ?? null;
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
		load(partId);
	});

	async function load(part: string) {
		error = null;
		try {
			const { parts } = await api.placement();
			const view = parts.find((p) => p.id === part);
			if (!view) {
				error = 'This part does not exist.';
				return;
			}
			if (view.status === 'done' && page.url.searchParams.get('retake') !== '1') {
				doneView = view;
				return;
			}
			await start(part);
		} catch (err) {
			error = message(err);
		}
	}

	async function start(part: string) {
		doneView = null;
		run = await api.startPlacement(part);
		shownAt = performance.now();
	}

	async function answer(response: ItemResponse) {
		if (!run?.next) return;
		submitting = true;
		error = null;
		try {
			result = await api.answerPlacement(run.runId, {
				itemId: run.next.id,
				response,
				latencyMs: Math.round(performance.now() - shownAt),
				consulted: glossary.consultedNow
			});
		} catch (err) {
			error = message(err);
		} finally {
			submitting = false;
		}
	}

	function next() {
		if (!result) return;
		run = result.state;
		result = null;
		glossary.resetConsulted();
		shownAt = performance.now();
		window.scrollTo({ top: 0 });
	}

	async function retake() {
		await goto(`?retake=1`, { replaceState: true });
		try {
			await start(partId);
		} catch (err) {
			error = message(err);
		}
	}

	function message(err: unknown): string {
		if (err instanceof ApiError) return `${err.status} · ${err.message}`;
		return 'Could not reach the API.';
	}

	const percent = $derived(
		run && run.progress.max > 0 ? Math.round((run.progress.answered / run.progress.max) * 100) : 0
	);
</script>

<header class="cabecera tapa">
	<a class="etiqueta volver" href="/">← Tablero</a>
	{#if run && run.status === 'in_progress'}
		<p class="etiqueta num">{run.progress.answered} / {run.progress.max}</p>
	{/if}
</header>

{#if run && run.status === 'in_progress'}
	<div class="barra" role="progressbar" aria-valuenow={percent} aria-valuemin="0" aria-valuemax="100">
		<span style:width="{percent}%"></span>
	</div>
{/if}

<main class="test tapa">
	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}

	{#if doneView}
		<p class="etiqueta">Obra 0 · Cimiento · {doneView.nameEn}</p>
		<h1 class="titular">Placement done.</h1>
		{#if doneView.summary}
			<PlacementSummary summary={doneView.summary} />
		{/if}
		{#if doneView.missed?.length}
			<MissedList missed={doneView.missed} />
		{/if}
		<button class="secundario etiqueta" type="button" onclick={retake}>Retake this part</button>
	{:else if run?.status === 'done' && !result}
		<p class="etiqueta">Obra 0 · Cimiento · {run.partName}</p>
		<h1 class="titular">Here's your map.</h1>
		{#if run.summary}
			<PlacementSummary summary={run.summary} />
		{/if}
		{#if run.missed?.length}
			<MissedList missed={run.missed} />
		{/if}
		<a class="primario" href="/">Back to the board</a>
	{:else if run?.next}
		<p class="etiqueta">{run.partName} · {run.nextSkill?.nameEn}</p>
		{#key run.next.id}
			<Exercise item={run.next} {result} {submitting} onsubmit={answer} />
		{/key}
		{#if result}
			<Feedback {result} consulted={glossary.consultedNow} onnext={next} last={result.state.status === 'done'} />
		{/if}
	{:else if !error}
		<p class="etiqueta">Loading</p>
	{/if}
</main>

<style>
	.cabecera {
		display: flex;
		justify-content: space-between;
		align-items: center;
		min-height: 44px;
	}

	.volver {
		text-decoration: none;
		min-height: 44px;
		display: inline-flex;
		align-items: center;
	}

	.barra {
		height: 3px;
		background: var(--line-soft);
		margin-bottom: calc(var(--module) * 1.5);
	}

	.barra span {
		display: block;
		height: 100%;
		background: var(--baranda);
		transition: width 350ms ease;
	}

	.test {
		display: grid;
		gap: 24px;
		max-width: 760px;
		padding-bottom: calc(var(--module) * 2);
	}

	.titular {
		font-size: clamp(40px, 8vw, 56px);
	}

	.error {
		color: var(--oxido);
		margin: 0;
	}

	.primario {
		justify-self: start;
		display: inline-flex;
		align-items: center;
		min-height: 52px;
		padding: 0 28px;
		background: var(--text);
		color: var(--bg);
		font-weight: 600;
		text-decoration: none;
	}

	.secundario {
		justify-self: start;
		min-height: 44px;
		background: none;
		border: 0;
		border-bottom: 1px solid var(--line);
		padding: 0;
		cursor: pointer;
		color: var(--text);
	}
</style>
