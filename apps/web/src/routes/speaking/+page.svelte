<script lang="ts">
	import { goto } from '$app/navigation';
	import { api, ApiError, type Drill, type SpeakingResult } from '$lib/api';
	import Recorder from '$lib/components/Recorder.svelte';
	import { session } from '$lib/session.svelte';

	let drill = $state<Drill | null>(null);
	let enabled = $state(true);
	let result = $state<SpeakingResult | null>(null);
	let sending = $state(false);
	let error = $state<string | null>(null);
	let loading = $state(true);
	let showHint = $state(false);
	let recorder = $state<ReturnType<typeof Recorder> | null>(null);

	$effect(() => {
		if (!session.ready) return;
		if (!session.user) {
			goto('/');
			return;
		}
		load();
	});

	async function load() {
		loading = true;
		error = null;
		result = null;
		showHint = false;
		try {
			const res = await api.speakingNext();
			drill = res.next;
			enabled = res.enabled;
		} catch (err) {
			error = message(err);
		} finally {
			loading = false;
		}
	}

	async function send(audio: Blob, seconds: number) {
		if (!drill) return;
		sending = true;
		error = null;
		try {
			result = await api.speakingAnswer(drill.id, audio, seconds);
		} catch (err) {
			error = message(err);
		} finally {
			sending = false;
		}
	}

	function next() {
		drill = result?.next ?? null;
		result = null;
		showHint = false;
		recorder?.reset();
		window.scrollTo({ top: 0 });
	}

	function message(err: unknown): string {
		if (err instanceof ApiError) {
			if (err.status === 503) return 'La práctica oral todavía no está configurada en el servidor.';
			return `${err.status} · ${err.message}`;
		}
		return 'No se pudo contactar la API.';
	}

	/** Marca en la transcripción los pronombres que delatan el error. */
	function pieces(text: string, wrong: string[]) {
		if (wrong.length === 0) return [{ text, bad: false }];
		const re = new RegExp(`\\b(${wrong.join('|')})\\b`, 'gi');
		const out: { text: string; bad: boolean }[] = [];
		let last = 0;
		for (const m of text.matchAll(re)) {
			if (m.index! > last) out.push({ text: text.slice(last, m.index), bad: false });
			out.push({ text: m[0], bad: true });
			last = m.index! + m[0].length;
		}
		if (last < text.length) out.push({ text: text.slice(last), bad: false });
		return out;
	}
</script>

<header class="cabecera tapa">
	<a class="etiqueta volver" href="/">← Tablero</a>
	{#if drill}
		<p class="etiqueta num">Speaking · {drill.left + 1} left</p>
	{/if}
</header>

<main class="hablar tapa">
	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}

	{#if loading}
		<p class="etiqueta">Loading</p>
	{:else if !enabled}
		<h1 class="titular">Speaking is off.</h1>
		<p class="bajada lectura">
			Falta la clave del proveedor de transcripción en el servidor. El resto de la app funciona igual.
		</p>
	{:else if !drill}
		<p class="etiqueta">Speaking · done</p>
		<h1 class="titular">That's the speaking for today.</h1>
		<a class="primario" href="/">Back to the board</a>
	{:else}
		<p class="etiqueta">{drill.skillEn}</p>
		<p class="contexto lectura" lang="en">{drill.context}</p>
		<h1 class="consigna" lang="en">{drill.prompt_en}</h1>

		{#if !result}
			<Recorder
				bind:this={recorder}
				seconds={drill.seconds}
				disabled={sending}
				onrecorded={send}
			/>
			{#if sending}
				<p class="etiqueta">Transcribiendo…</p>
			{/if}

			{#if drill.hint_es}
				<button class="etiqueta explicar" type="button" onclick={() => (showHint = !showHint)}>
					{showHint ? 'Ocultar la pista' : 'Dame una pista'}
				</button>
				{#if showHint}
					<p class="pista lectura" lang="es">{drill.hint_es}</p>
				{/if}
			{/if}
		{:else}
			<section class="resultado" aria-live="polite">
				<p class="veredicto" class:ok={result.correct} class:mal={!result.correct}>
					{#if result.tooShort}
						Muy corto para medir.
					{:else if result.correct}
						Correct.
					{:else if result.wrong?.length}
						Se te escapó "{result.wrong.join('", "')}".
					{:else}
						Faltó usar "{(result.missing ?? []).join('", "')}".
					{/if}
				</p>

				<p class="etiqueta">Lo que dijiste</p>
				<p class="transcripcion" lang="en">
					{#each pieces(result.transcript, result.wrong ?? []) as part (part.text + part.bad)}<span
							class:mal={part.bad}>{part.text}</span
						>{/each}
				</p>

				{#if result.hintEs}
					<p class="pista lectura" lang="es">{result.hintEs}</p>
				{/if}

				<div class="acciones">
					<button class="primario" type="button" onclick={next}>
						{result.next ? 'Next drill' : 'Done'}
					</button>
					{#if result.colada > 0}
						<p class="etiqueta num">+{result.colada} colada</p>
					{/if}
				</div>
			</section>
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

	.hablar {
		display: grid;
		gap: 20px;
		max-width: 700px;
		padding-block: calc(var(--module) * 1.5) calc(var(--module) * 2);
	}

	p {
		margin: 0;
	}

	.titular {
		font-size: clamp(36px, 7vw, 52px);
	}

	.contexto {
		color: var(--text-muted);
	}

	.consigna {
		margin: 0;
		font-family: var(--font-mono);
		font-size: 21px;
		font-weight: 400;
		line-height: 1.6;
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

	.pista {
		padding: 16px;
		background: var(--surface);
	}

	.resultado {
		display: grid;
		gap: 12px;
		padding-top: 20px;
		border-top: 1px solid var(--line);
	}

	.veredicto {
		font-weight: 800;
		font-size: 24px;
		letter-spacing: -0.02em;
	}

	.veredicto.ok {
		color: var(--baranda);
	}

	.veredicto.mal {
		color: var(--oxido);
	}

	.transcripcion {
		font-family: var(--font-mono);
		font-size: 17px;
		line-height: 1.8;
		padding: 16px;
		background: var(--surface);
	}

	.transcripcion .mal {
		color: var(--oxido);
		text-decoration: underline wavy;
		text-underline-offset: 4px;
	}

	.acciones {
		display: flex;
		align-items: center;
		gap: 20px;
		flex-wrap: wrap;
	}

	.primario {
		justify-self: start;
		display: inline-flex;
		align-items: center;
		min-height: 52px;
		padding: 0 28px;
		background: var(--text);
		color: var(--bg);
		border: 0;
		font-weight: 600;
		text-decoration: none;
		cursor: pointer;
	}

	.error {
		color: var(--oxido);
	}
</style>
