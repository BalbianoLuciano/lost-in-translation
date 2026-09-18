<script lang="ts">
	import type { ItemResponse, PublicItem } from '$lib/api';

	/** Lo que el ejercicio necesita de la corrección, venga del diagnóstico o de la práctica. */
	type Corrected = { correct: boolean; expected: string; wrongIndex?: number; partOk?: boolean };

	type Props = {
		item: PublicItem;
		/** Con resultado, el ejercicio queda bloqueado. */
		result: Corrected | null;
		submitting: boolean;
		onsubmit: (response: ItemResponse) => void;
	};
	let { item, result, submitting, onsubmit }: Props = $props();

	let text = $state('');
	let choice = $state<number | null>(null);
	let tokenIndex = $state<number | null>(null);
	let input = $state<HTMLInputElement | null>(null);

	const locked = $derived(result !== null || submitting);

	const ready = $derived.by(() => {
		switch (item.type) {
			case 'cloze':
				return text.trim() !== '';
			case 'choice':
			case 'explain_why':
				return choice !== null;
			case 'fix_error':
				return tokenIndex !== null && text.trim() !== '';
		}
	});

	const clozeParts = $derived(item.type === 'cloze' ? item.text.split('___') : []);

	const focusParts = $derived.by(() => {
		if (item.type !== 'explain_why' || !item.focus) return null;
		const i = item.text.indexOf(item.focus);
		return [item.text.slice(0, i), item.focus, item.text.slice(i + item.focus.length)];
	});

	function submit() {
		if (!ready || locked) return;
		if (item.type === 'cloze') onsubmit({ text });
		else if (item.type === 'fix_error') onsubmit({ tokenIndex: tokenIndex!, text });
		else onsubmit({ choice: choice! });
	}

	function pickToken(i: number) {
		if (locked) return;
		tokenIndex = i;
		queueMicrotask(() => input?.focus());
	}

	function pickOption(i: number) {
		if (locked) return;
		choice = i;
	}

	function onkeydown(e: KeyboardEvent) {
		if (locked || !item.options) return;
		const n = Number(e.key);
		if (Number.isInteger(n) && n >= 1 && n <= item.options.length) {
			const target = e.target as HTMLElement | null;
			if (target?.tagName === 'INPUT') return;
			choice = n - 1;
		}
	}

	function optionClass(i: number): string {
		if (!result) return choice === i ? 'elegida' : '';
		const expected = item.options?.[i] === result.expected;
		if (expected) return 'correcta';
		if (choice === i) return 'equivocada';
		return 'apagada';
	}

	function tokenClass(i: number): string {
		if (!result) return tokenIndex === i ? 'elegida' : '';
		if (i === result.wrongIndex) return 'correcta';
		if (i === tokenIndex) return 'equivocada';
		return '';
	}
</script>

<svelte:window {onkeydown} />

<form
	class="ejercicio"
	data-item={item.id}
	onsubmit={(e) => {
		e.preventDefault();
		submit();
	}}
>
	{#if item.context}
		<p class="contexto">{item.context}</p>
	{/if}

	{#if item.type === 'cloze'}
		<p class="oracion" lang="en">
			{clozeParts[0]}<label class="hueco" class:correcta={result?.correct} class:equivocada={result && !result.correct}>
				<span class="visually-hidden">Answer</span>
				<input
					bind:value={text}
					disabled={locked}
					size={Math.max(8, text.length + 1)}
					autocomplete="off"
					autocapitalize="off"
					spellcheck="false"
					enterkeyhint="done"
					{@attach (el: HTMLInputElement) => el.focus()}
				/>
			</label>{clozeParts[1]}
		</p>
	{:else if item.type === 'fix_error'}
		<p class="consigna">Tap the word that's wrong, then write the right one.</p>
		<p class="oracion tokens" lang="en">
			{#each item.tokens ?? [] as token, i (i)}
				<button type="button" class="token {tokenClass(i)}" disabled={locked} onclick={() => pickToken(i)}
					>{token}</button
				>{' '}
			{/each}
		</p>
		{#if tokenIndex !== null}
			<label class="correccion">
				<span class="etiqueta">Correct word</span>
				<input
					bind:this={input}
					bind:value={text}
					disabled={locked}
					autocomplete="off"
					autocapitalize="off"
					spellcheck="false"
					enterkeyhint="done"
				/>
			</label>
		{/if}
	{:else}
		{#if item.type === 'explain_why' && focusParts}
			<p class="oracion" lang="en">
				{focusParts[0]}<mark>{focusParts[1]}</mark>{focusParts[2]}
			</p>
			<p class="pregunta">{item.question}</p>
		{:else}
			<p class="oracion" lang="en">{item.text}</p>
		{/if}

		<ol class="opciones" class:reglas={item.type === 'explain_why'}>
			{#each item.options ?? [] as option, i (i)}
				<li>
					<button type="button" class="opcion {optionClass(i)}" disabled={locked} onclick={() => pickOption(i)}>
						<span class="etiqueta num">{i + 1}</span>
						<span lang="en">{option}</span>
					</button>
				</li>
			{/each}
		</ol>
	{/if}

	{#if !result}
		<button class="primario" type="submit" disabled={!ready || locked}>
			{submitting ? 'Checking…' : 'Check'}
		</button>
	{/if}
</form>

<style>
	.ejercicio {
		display: grid;
		gap: 20px;
	}

	.contexto,
	.consigna,
	.pregunta {
		margin: 0;
		color: var(--text-muted);
	}

	.pregunta {
		color: var(--text);
		font-weight: 600;
	}

	.oracion {
		margin: 0;
		font-family: var(--font-mono);
		font-size: 19px;
		line-height: 1.9;
	}

	@media (min-width: 900px) {
		.oracion {
			font-size: 22px;
		}
	}

	input {
		font: inherit;
		color: var(--text);
		background: var(--surface);
		border: 0;
		border-bottom: 2px solid var(--text);
		padding: 2px 6px;
		border-radius: 0;
	}

	input:focus {
		outline: none;
		border-bottom-color: var(--baranda);
	}

	.hueco input {
		font-family: var(--font-mono);
		min-width: 8ch;
		max-width: 100%;
	}

	.hueco.correcta input {
		border-bottom-color: var(--baranda);
		color: var(--baranda);
	}

	.hueco.equivocada input {
		border-bottom-color: var(--oxido);
		color: var(--oxido);
		text-decoration: line-through;
	}

	mark {
		background: none;
		color: inherit;
		text-decoration: underline 2px var(--baranda);
		text-underline-offset: 6px;
	}

	.tokens {
		line-height: 2.4;
	}

	.token {
		font: inherit;
		background: none;
		border: 0;
		border-bottom: 1px dashed var(--line);
		padding: 2px 1px;
		cursor: pointer;
	}

	.token:disabled {
		cursor: default;
	}

	.token.elegida {
		background: var(--text);
		color: var(--bg);
		border-bottom-color: transparent;
	}

	.token.correcta {
		color: var(--baranda);
		border-bottom: 2px solid var(--baranda);
	}

	.token.equivocada {
		color: var(--oxido);
		text-decoration: line-through;
	}

	.correccion {
		display: grid;
		gap: 6px;
		max-width: 320px;
	}

	.correccion input {
		font-family: var(--font-mono);
		font-size: 19px;
	}

	.opciones {
		list-style: none;
		margin: 0;
		padding: 0;
		border-top: 1px solid var(--line);
	}

	.opcion {
		width: 100%;
		min-height: 52px;
		display: grid;
		grid-template-columns: 28px 1fr;
		align-items: baseline;
		gap: 12px;
		padding: 12px 8px;
		background: none;
		border: 0;
		border-bottom: 1px solid var(--line-soft);
		text-align: left;
		cursor: pointer;
	}

	.opciones:not(.reglas) .opcion span[lang] {
		font-family: var(--font-mono);
	}

	.opcion:hover:not(:disabled) {
		background: var(--surface);
	}

	.opcion:disabled {
		cursor: default;
	}

	.opcion.elegida {
		background: var(--text);
		color: var(--bg);
	}

	.opcion.elegida .etiqueta {
		color: var(--bg);
	}

	.opcion.correcta {
		color: var(--baranda);
		font-weight: 600;
	}

	.opcion.correcta span[lang] {
		text-decoration: underline 2px;
		text-underline-offset: 5px;
	}

	.opcion.equivocada {
		color: var(--oxido);
	}

	.opcion.equivocada span[lang] {
		text-decoration: line-through;
	}

	.opcion.apagada {
		color: var(--text-muted);
	}

	.primario {
		justify-self: start;
		min-height: 52px;
		padding: 0 28px;
		background: var(--text);
		color: var(--bg);
		border: 0;
		font-weight: 600;
		cursor: pointer;
	}

	.primario:disabled {
		opacity: 0.4;
		cursor: default;
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
