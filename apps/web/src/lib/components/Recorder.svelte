<script lang="ts">
	import { onDestroy } from 'svelte';

	type Props = {
		/** Tope de grabación; se corta sola al llegar. */
		seconds: number;
		disabled?: boolean;
		onrecorded: (audio: Blob, seconds: number) => void;
	};
	let { seconds, disabled = false, onrecorded }: Props = $props();

	type Estado = 'idle' | 'asking' | 'recording' | 'done';
	let estado = $state<Estado>('idle');
	let elapsed = $state(0);
	let error = $state<string | null>(null);

	let recorder: MediaRecorder | null = null;
	let chunks: Blob[] = [];
	let timer: ReturnType<typeof setInterval> | null = null;
	let startedAt = 0;

	// El formato depende del navegador: Chrome graba webm y Safari mp4.
	function mimeType(): string {
		for (const type of ['audio/webm;codecs=opus', 'audio/webm', 'audio/mp4']) {
			if (typeof MediaRecorder !== 'undefined' && MediaRecorder.isTypeSupported(type)) return type;
		}
		return '';
	}

	async function start() {
		error = null;
		estado = 'asking';
		try {
			const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
			const type = mimeType();
			recorder = new MediaRecorder(stream, type ? { mimeType: type } : undefined);
			chunks = [];
			recorder.ondataavailable = (e) => e.data.size > 0 && chunks.push(e.data);
			recorder.onstop = () => {
				stream.getTracks().forEach((t) => t.stop());
				const audio = new Blob(chunks, { type: recorder?.mimeType || 'audio/webm' });
				estado = 'done';
				onrecorded(audio, Math.round((performance.now() - startedAt) / 1000));
			};
			recorder.start();
			startedAt = performance.now();
			elapsed = 0;
			estado = 'recording';
			timer = setInterval(() => {
				elapsed = Math.round((performance.now() - startedAt) / 1000);
				if (elapsed >= seconds) stop();
			}, 200);
		} catch {
			estado = 'idle';
			error = 'No se pudo usar el micrófono. Revisá el permiso del navegador.';
		}
	}

	function stop() {
		if (timer) clearInterval(timer);
		timer = null;
		recorder?.state === 'recording' && recorder.stop();
	}

	onDestroy(() => {
		if (timer) clearInterval(timer);
		if (recorder?.state === 'recording') recorder.stop();
	});

	export function reset() {
		estado = 'idle';
		elapsed = 0;
	}

	const restante = $derived(Math.max(seconds - elapsed, 0));
</script>

<div class="grabador">
	{#if estado === 'recording'}
		<button class="boton grabando" type="button" onclick={stop} aria-label="Parar la grabación">
			<span class="punto"></span>
		</button>
		<p class="etiqueta num" aria-live="polite">{restante}s · tocá para terminar</p>
	{:else}
		<button class="boton" type="button" onclick={start} disabled={disabled || estado !== 'idle'}>
			<span class="visually-hidden">Grabar</span>
			<span class="mic" aria-hidden="true"></span>
		</button>
		<p class="etiqueta num">
			{#if estado === 'asking'}
				Pidiendo el micrófono…
			{:else if estado === 'done'}
				Listo
			{:else}
				Hasta {seconds}s
			{/if}
		</p>
	{/if}

	{#if error}
		<p class="error" role="alert">{error}</p>
	{/if}
</div>

<style>
	.grabador {
		display: grid;
		justify-items: center;
		gap: 10px;
	}

	.boton {
		width: 84px;
		height: 84px;
		display: grid;
		place-items: center;
		background: var(--text);
		border: 0;
		cursor: pointer;
		box-shadow: 4px 4px 0 0 var(--line);
	}

	.boton:disabled {
		opacity: 0.5;
		cursor: default;
	}

	/* El micrófono, dibujado: un rectángulo con su pie, sin íconos de librería */
	.mic {
		width: 18px;
		height: 30px;
		background: var(--bg);
		border-radius: 9px 9px 9px 9px;
		position: relative;
	}

	.mic::after {
		content: '';
		position: absolute;
		left: 50%;
		bottom: -10px;
		width: 2px;
		height: 10px;
		background: var(--bg);
		transform: translateX(-50%);
	}

	.grabando {
		background: var(--oxido);
		animation: latir 1.6s ease-in-out infinite;
	}

	.punto {
		width: 24px;
		height: 24px;
		background: var(--bg);
	}

	@keyframes latir {
		0%,
		100% {
			box-shadow: 4px 4px 0 0 var(--line);
		}
		50% {
			box-shadow: 4px 4px 0 0 var(--oxido);
		}
	}

	.error {
		margin: 0;
		color: var(--oxido);
		text-align: center;
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
