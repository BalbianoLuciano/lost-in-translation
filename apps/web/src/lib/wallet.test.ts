import { describe, expect, it } from 'vitest';
import { conDireccion, MINT_ABI } from './wallet.svelte';

// El mensaje que emite el servidor, con la segunda línea vacía de dirección:
// el desafío se pide antes de que la billetera diga quién es.
const mensaje = [
	'localhost:5173 wants you to sign in with your Ethereum account:',
	'0x0000000000000000000000000000000000000000',
	'',
	'Link this wallet to your Lost in Translation account.',
	'',
	'URI: http://localhost:5173',
	'Version: 1',
	'Chain ID: 84532',
	'Nonce: c0ffee00c0ffee00',
	'Issued At: 2026-09-24T12:00:00Z',
	'Expiration Time: 2026-09-24T12:10:00Z'
].join('\n');

describe('conDireccion', () => {
	it('pone la dirección en la segunda línea y no toca nada más', () => {
		const salida = conDireccion(mensaje, '0x70997970C51812dc3A010C7d01b50e0d17dc79C8');
		const lineas = salida.split('\n');

		expect(lineas[1]).toBe('0x70997970c51812dc3a010c7d01b50e0d17dc79c8');
		expect(lineas.length).toBe(11);
		// El resto del texto tiene que quedar byte por byte igual: es lo que se
		// firma, y el servidor lo compara con lo que él mismo emitió.
		expect([...lineas.slice(0, 1), ...lineas.slice(2)]).toEqual([
			...mensaje.split('\n').slice(0, 1),
			...mensaje.split('\n').slice(2)
		]);
	});

	it('la baja a minúsculas, que es como la guarda el servidor', () => {
		// Una billetera devuelve la dirección con el checksum de EIP-55. Si
		// viajara con mayúsculas, el UNIQUE de la base dejaría de significar
		// "una billetera, una persona".
		const conMayusculas = conDireccion(mensaje, '0xAB00000000000000000000000000000000000000');
		expect(conMayusculas.split('\n')[1]).toBe('0xab00000000000000000000000000000000000000');
	});
});

describe('el ABI', () => {
	// Sólo mint: el resto del contrato la web no lo llama, y traer artefactos de
	// compilación de Solidity para copiar cuatro tipos no se justifica.
	it('tiene exactamente la firma que espera el contrato', () => {
		expect(MINT_ABI).toEqual(['function mint(address to, uint16 pieza, uint64 deadline, bytes firma)']);
	});
});
