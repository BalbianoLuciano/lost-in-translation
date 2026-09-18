<script lang="ts">
	import { buildPilar, tensors, type Piece } from '$lib/pilar';

	type Props = {
		pieces: Piece[];
		/** Rótulos al costado (desktop) o debajo al tocar (mobile). */
		selected?: string | null;
		onselect?: (id: string) => void;
		/** Ancho en pantalla; el alto sale de las piezas, para no achatar la obra. */
		width?: number;
	};
	let { pieces, selected = null, onselect, width = 200 }: Props = $props();

	const pilar = $derived(buildPilar(pieces));
	// Aire arriba y abajo para que la sombra y la oscilación no se corten.
	const pad = 6;
</script>

<figure class="pilar" style:--ancho="{width}px">
	<svg
		viewBox="0 0 {pilar.width} {pilar.height + pad * 2}"
		preserveAspectRatio="xMidYMid meet"
		role="img"
		aria-label="Tu obra: una pieza por tema"
	>
		<defs>
			<!-- El grano del hormigón: sin esto, una pieza se lee como un rectángulo de color -->
			<filter id="grano">
				<feTurbulence type="fractalNoise" baseFrequency="0.9" numOctaves="3" result="ruido" />
				<feColorMatrix in="ruido" type="saturate" values="0" result="gris" />
				<feComposite operator="in" in="gris" in2="SourceGraphic" result="textura" />
				<feBlend in="SourceGraphic" in2="textura" mode="multiply" />
			</filter>
		</defs>

		<g transform="translate(0 {pad})">
			{#each pilar.shapes as shape (shape.id)}
				<g class="pieza {shape.state}" class:elegida={selected === shape.id}>
					<!-- Flota el dibujo; la zona de toque queda quieta debajo del dedo -->
					<g class="flota" style:--retraso="{(shape.top % 7) * 0.35}s">
					<path
						class="cara"
						d={shape.path}
						filter={shape.state === 'plano' ? undefined : 'url(#grano)'}
					/>

					{#each tensors(shape, pilar.width) as t (t.cx)}
						<circle class="tensor" cx={t.cx} cy={t.cy} r="2.2" />
					{/each}

					{#if shape.state === 'oxidada'}
						<!-- El óxido chorrea vertical desde arriba, como en el hormigón real -->
						{#each [0.3, 0.55] as x (x)}
							<rect
								class="oxido"
								x={x * pilar.width}
								y={shape.top + 2}
								width="3"
								height={shape.height * 0.75}
							/>
						{/each}
					{/if}

					</g>

					{#if onselect}
						<!-- Zona táctil: el path fino sería imposible de tocar en el celu -->
						<rect
							class="toque"
							x="0"
							y={shape.top}
							width={pilar.width}
							height={shape.height}
							role="button"
							tabindex="0"
							aria-label={shape.name}
							onclick={() => onselect?.(shape.id)}
							onkeydown={(e) => (e.key === 'Enter' || e.key === ' ') && onselect?.(shape.id)}
						/>
					{/if}
				</g>
			{/each}
		</g>
	</svg>
</figure>

<style>
	.pilar {
		margin: 0;
		display: grid;
		justify-items: center;
	}

	svg {
		width: min(var(--ancho), 100%);
		height: auto;
		overflow: visible;
	}

	.cara {
		stroke: var(--line);
		stroke-width: 1.25;
		fill: none;
	}

	/* Plano: la pieza está dibujada pero no construida */
	.plano .cara {
		stroke-dasharray: 4 4;
		opacity: 0.7;
	}

	.suspendida .cara {
		fill: var(--hormigon);
	}

	.calzada .cara {
		fill: var(--hormigon-luz);
	}

	.oxidada .cara {
		fill: var(--hormigon);
	}

	.tensor {
		fill: var(--hormigon-sombra);
		opacity: 0.55;
	}

	.oxido {
		fill: var(--oxido);
		opacity: 0.45;
	}

	.toque {
		fill: transparent;
		cursor: pointer;
	}

	.elegida .cara {
		stroke: var(--baranda);
		stroke-width: 2.5;
	}

	/* Lo que no está calzado, levita: el movimiento dice que todavía no apoya */
	.suspendida .flota,
	.oxidada .flota,
	.plano .flota {
		animation: levitar 5.5s ease-in-out infinite;
		animation-delay: var(--retraso);
	}

	@keyframes levitar {
		0%,
		100% {
			transform: translateY(-1.5px);
		}
		50% {
			transform: translateY(1.5px);
		}
	}
</style>
