/**
 * La geometría del pilar (design.md §7.1).
 *
 * Cada tema es una pieza. Las piezas vecinas comparten una junta única —la misma
 * línea de los dos lados, por eso calzan— y la separación entre ellas sale del
 * dominio: lo que no sabés flota, lo que dominás apoya.
 *
 * El algoritmo viene del pilar del carrusel de Gridwright, adaptado: allá el
 * alto de cada pieza era el de una sección de la página; acá es cuánta práctica
 * tiene el tema.
 */

export type PieceState = 'plano' | 'suspendida' | 'calzada' | 'oxidada';

export type Piece = {
	id: string;
	name: string;
	state: PieceState;
	/** 0 a 1: cuánto lo dominás. Define cuánto flota la pieza. */
	mastery: number;
	/** Cuántos ejercicios de práctica tiene el tema. */
	items: number;
};

export type Shape = Piece & {
	path: string;
	/** Coordenada del centro de la pieza, para poner el rótulo. */
	labelY: number;
	top: number;
	height: number;
	/** Cuánto está separada de la pieza de abajo. */
	gap: number;
};

export type Pilar = {
	width: number;
	height: number;
	shapes: Shape[];
};

type Seg = { t: 'L'; x: number; y: number } | { t: 'Q'; cx: number; cy: number; x: number; y: number };
type Joint = { x: number; y: number; segs: Seg[] };

/**
 * Los perfiles de junta, en coordenadas normalizadas: x de 0 a 1, y en unidades
 * de amplitud (negativo = sube). Cada par de piezas usa uno distinto.
 */
const JOINTS: Joint[] = [
	{ x: 0, y: 0, segs: [{ t: 'L', x: 1, y: 0 }] }, // recta
	{ x: 0, y: -0.7, segs: [{ t: 'L', x: 1, y: 0.7 }] }, // diagonal
	{ x: 0, y: 0, segs: [{ t: 'Q', cx: 0.5, cy: 2, x: 1, y: 0 }] }, // arco hacia abajo
	{
		x: 0,
		y: 0,
		segs: [
			{ t: 'L', x: 0.34, y: 0 },
			{ t: 'L', x: 0.34, y: -1 },
			{ t: 'L', x: 0.66, y: -1 },
			{ t: 'L', x: 0.66, y: 0 },
			{ t: 'L', x: 1, y: 0 }
		]
	}, // espiga
	{ x: 0, y: 0, segs: [{ t: 'Q', cx: 0.28, cy: -2, x: 1, y: 0 }] }, // arco corrido
	{
		x: 0,
		y: -0.6,
		segs: [
			{ t: 'L', x: 0.52, y: -0.6 },
			{ t: 'L', x: 0.52, y: 0.6 },
			{ t: 'L', x: 1, y: 0.6 }
		]
	}, // escalón
	{ x: 0, y: 0, segs: [{ t: 'Q', cx: 0.5, cy: -2, x: 1, y: 0 }] }, // arco
	{
		x: 0,
		y: 0,
		segs: [
			{ t: 'L', x: 0.4, y: 0 },
			{ t: 'L', x: 0.4, y: 1 },
			{ t: 'L', x: 0.72, y: 1 },
			{ t: 'L', x: 0.72, y: 0 },
			{ t: 'L', x: 1, y: 0 }
		]
	}, // espiga baja
	{ x: 0, y: 0, segs: [{ t: 'Q', cx: 0.74, cy: 2, x: 1, y: 0 }] }, // arco corrido al otro lado
	{ x: 0, y: 0.7, segs: [{ t: 'L', x: 1, y: -0.7 }] } // diagonal inversa
];

const FLAT: Joint = JOINTS[0];

export type Options = {
	width?: number;
	/** Separación máxima entre piezas, para un tema que no sabés nada. */
	gapMax?: number;
	/** Alto mínimo de una pieza, aunque tenga poca práctica. */
	minHeight?: number;
	heightPerItem?: number;
};

// El pilar es una columna alta y angosta, como el de Gridwright: si se lo achata
// para que entre en una caja baja, las piezas se leen como un código de barras.
const DEFAULTS: Required<Options> = { width: 216, gapMax: 20, minHeight: 26, heightPerItem: 2.4 };

/** Cuánto flota la pieza: lo que no dominás, más separado. */
export function gapFor(piece: Piece, gapMax: number): number {
	if (piece.state === 'calzada') return 0;
	const mastery = Math.min(Math.max(piece.mastery, 0), 1);
	return Math.round((1 - mastery) * gapMax);
}

function height(piece: Piece, o: Required<Options>): number {
	return Math.round(o.minHeight + piece.items * o.heightPerItem);
}

/** El punto donde termina un perfil. */
function endOf(joint: Joint): { x: number; y: number } {
	const last = joint.segs[joint.segs.length - 1];
	return { x: last.x, y: last.y };
}

/** Recorre un perfil de izquierda a derecha sobre la línea base `y`. */
function trace(joint: Joint, width: number, y: number, amp: number): string {
	const p = (x: number, dy: number) => `${(x * width).toFixed(2)} ${(y + dy * amp).toFixed(2)}`;
	return joint.segs
		.map((s) => (s.t === 'Q' ? `Q${p(s.cx, s.cy)} ${p(s.x, s.y)}` : `L${p(s.x, s.y)}`))
		.join(' ');
}

/** El mismo perfil, de derecha a izquierda: así la junta calza con la de arriba. */
function traceBack(joint: Joint, width: number, y: number, amp: number): string {
	const p = (x: number, dy: number) => `${(x * width).toFixed(2)} ${(y + dy * amp).toFixed(2)}`;
	const points = [{ x: joint.x, y: joint.y }, ...joint.segs.map((s) => ({ x: s.x, y: s.y }))];
	let d = '';
	for (let k = joint.segs.length - 1; k >= 0; k--) {
		const seg = joint.segs[k];
		const to = points[k];
		d += seg.t === 'Q' ? `Q${p(seg.cx, seg.cy)} ${p(to.x, to.y)} ` : `L${p(to.x, to.y)} `;
	}
	return d.trim();
}

/** La amplitud de una junta, acotada por la pieza más chica de las dos. */
function amplitude(a: number, b: number): number {
	return Math.min(0.28 * Math.min(a, b), 34);
}

export function buildPilar(pieces: Piece[], options: Options = {}): Pilar {
	const o = { ...DEFAULTS, ...options };
	if (pieces.length === 0) return { width: o.width, height: 0, shapes: [] };

	const heights = pieces.map((p) => height(p, o));
	const gaps = pieces.map((p, i) => (i === pieces.length - 1 ? 0 : gapFor(p, o.gapMax)));
	const amps = heights.map((h, i) => (i === heights.length - 1 ? 0 : amplitude(h, heights[i + 1])));

	const shapes: Shape[] = [];
	let y = 0;
	for (let i = 0; i < pieces.length; i++) {
		const top = y;
		const bottom = y + heights[i];
		const above = i > 0 ? JOINTS[(i - 1) % JOINTS.length] : FLAT;
		const below = i < pieces.length - 1 ? JOINTS[i % JOINTS.length] : FLAT;
		const ampAbove = i > 0 ? amps[i - 1] : 0;
		const ampBelow = amps[i];

		const start = { x: above.x, y: above.y };
		const end = endOf(below);
		const path =
			`M${(start.x * o.width).toFixed(2)} ${(top + start.y * ampAbove).toFixed(2)} ` +
			`${trace(above, o.width, top, ampAbove)} ` +
			`L${(end.x * o.width).toFixed(2)} ${(bottom + end.y * ampBelow).toFixed(2)} ` +
			`${traceBack(below, o.width, bottom, ampBelow)} Z`;

		shapes.push({
			...pieces[i],
			path,
			top,
			height: heights[i],
			gap: gaps[i],
			labelY: top + heights[i] / 2
		});
		y = bottom + gaps[i];
	}

	return { width: o.width, height: Math.round(y), shapes };
}

/** Los mechinales: la huella del proceso constructivo, sólo en piezas grandes. */
export function tensors(shape: Shape, width: number): { cx: number; cy: number }[] {
	if (shape.height < 44 || shape.state === 'plano') return [];
	const cy = shape.top + shape.height * 0.62;
	return [0.22, 0.42, 0.62, 0.82].map((x) => ({ cx: x * width, cy }));
}
