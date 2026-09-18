import { describe, expect, it } from 'vitest';
import { buildPilar, gapFor, tensors, type Piece } from './pilar';

const piece = (id: string, state: Piece['state'], mastery = 0.5, items = 10): Piece => ({
	id,
	name: id,
	state,
	mastery,
	items
});

describe('gapFor', () => {
	it('lo calzado apoya: sin separación', () => {
		expect(gapFor(piece('a', 'calzada', 0.9), 26)).toBe(0);
	});

	it('lo que menos dominás, más flota', () => {
		const poco = gapFor(piece('a', 'plano', 0.1), 26);
		const casi = gapFor(piece('b', 'suspendida', 0.8), 26);
		expect(poco).toBeGreaterThan(casi);
	});

	it('aguanta dominios fuera de rango', () => {
		expect(gapFor(piece('a', 'plano', -1), 26)).toBe(26);
		expect(gapFor(piece('a', 'plano', 2), 26)).toBe(0);
	});
});

describe('buildPilar', () => {
	it('sin piezas no hay pilar', () => {
		expect(buildPilar([]).shapes).toEqual([]);
	});

	it('apila las piezas en orden y sin superponerlas', () => {
		const { shapes, height } = buildPilar([
			piece('a', 'calzada', 0.9),
			piece('b', 'suspendida', 0.6),
			piece('c', 'plano', 0.1)
		]);
		expect(shapes.map((s) => s.id)).toEqual(['a', 'b', 'c']);
		for (let i = 1; i < shapes.length; i++) {
			expect(shapes[i].top).toBeGreaterThanOrEqual(shapes[i - 1].top + shapes[i - 1].height);
		}
		expect(height).toBeGreaterThan(0);
	});

	it('el tema con más práctica ocupa más', () => {
		const { shapes } = buildPilar([piece('corto', 'plano', 0.2, 4), piece('largo', 'plano', 0.2, 20)]);
		expect(shapes[1].height).toBeGreaterThan(shapes[0].height);
	});

	it('una pieza calzada queda pegada a la de abajo', () => {
		const { shapes } = buildPilar([piece('a', 'calzada', 0.9), piece('b', 'calzada', 0.9)]);
		expect(shapes[0].gap).toBe(0);
		expect(shapes[1].top).toBe(shapes[0].top + shapes[0].height);
	});

	it('es determinista: el mismo pilar sale igual siempre', () => {
		const pieces = [piece('a', 'plano'), piece('b', 'suspendida'), piece('c', 'calzada', 0.9)];
		expect(buildPilar(pieces)).toEqual(buildPilar(pieces));
	});

	it('cada pieza genera un path cerrado', () => {
		const { shapes } = buildPilar([piece('a', 'plano'), piece('b', 'plano')]);
		for (const s of shapes) {
			expect(s.path.startsWith('M')).toBe(true);
			expect(s.path.endsWith('Z')).toBe(true);
			expect(s.path).not.toContain('NaN');
		}
	});

	it('las juntas vecinas son distintas entre sí', () => {
		const pieces = Array.from({ length: 5 }, (_, i) => piece(`p${i}`, 'plano'));
		const paths = buildPilar(pieces).shapes.map((s) => s.path);
		expect(new Set(paths).size).toBe(paths.length);
	});
});

describe('tensors', () => {
	it('los mechinales aparecen sólo en piezas con materia', () => {
		const { shapes, width } = buildPilar([piece('grande', 'calzada', 0.9, 20), piece('plana', 'plano', 0, 20)]);
		expect(tensors(shapes[0], width).length).toBe(4);
		expect(tensors(shapes[1], width)).toEqual([]);
	});

	it('una pieza chica no lleva mechinales', () => {
		const { shapes, width } = buildPilar([piece('chica', 'calzada', 0.9, 1)]);
		expect(tensors(shapes[0], width)).toEqual([]);
	});
});
