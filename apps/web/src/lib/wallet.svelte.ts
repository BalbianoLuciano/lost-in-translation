import { api, ApiError, type Mint, type WalletState } from '$lib/api';

/**
 * La billetera del navegador, y el reclamo de una distinción.
 *
 * Tres cosas y ninguna más:
 *
 *  1. conectar (una vez) — el servidor emite un mensaje EIP-4361, la billetera
 *     lo firma, y el servidor recupera la dirección del texto firmado;
 *  2. reclamar — el servidor firma un voucher, la billetera manda la
 *     transacción y la paga;
 *  3. confirmar — el txHash vuelve al servidor, que consulta el recibo.
 *
 * El servidor nunca manda la transacción: no tiene con qué, su clave firma y no
 * gasta. Y nada de esto existe si el servidor no tiene la cadena configurada:
 * la pantalla se ve igual que antes.
 *
 * viem y no ethers: los tipos salen del ABI y el bundle entra sólo cuando
 * alguien aprieta el botón, porque las importaciones son dinámicas.
 */

/**
 * El ABI, escrito a mano. Es la única función del contrato que la web llama, y
 * escribirla acá evita traer artefactos de compilación de Solidity a un
 * proyecto de SvelteKit para copiar cuatro tipos.
 */
export const MINT_ABI = [
	'function mint(address to, uint16 pieza, uint64 deadline, bytes firma)'
] as const;

/** Lo que inyecta una billetera de navegador (EIP-1193). */
type Proveedor = {
	request(args: { method: string; params?: unknown[] }): Promise<unknown>;
};

function proveedor(): Proveedor | null {
	if (typeof window === 'undefined') return null;
	return (window as unknown as { ethereum?: Proveedor }).ethereum ?? null;
}

/** Si hay una billetera instalada. Sin ella el botón de conectar no tiene sentido. */
export const hayBilletera = () => proveedor() !== null;

const mensajeDeError = (err: unknown, porDefecto: string): string => {
	if (err instanceof ApiError) return err.message;
	// 4001 es "el usuario rechazó": no es una falla, es un no.
	const code = (err as { code?: number })?.code;
	if (code === 4001) return 'You cancelled the request.';
	const short = (err as { shortMessage?: string })?.shortMessage;
	return short ?? (err instanceof Error ? err.message : porDefecto);
};

class Wallet {
	/** Lo que el servidor sabe: la dirección vinculada y lo que ya se acuñó. */
	estado = $state<WalletState | null>(null);
	/** El código de la distinción que se está reclamando, para deshabilitar su botón. */
	reclamando = $state<string | null>(null);
	conectando = $state(false);
	error = $state<string | null>(null);

	get direccion(): string | null {
		return this.estado?.address ?? null;
	}

	/** El recibo de una distinción, si hay. */
	mint(code: string): Mint | undefined {
		return this.estado?.mints.find((m) => m.code === code);
	}

	async cargar(): Promise<void> {
		try {
			this.estado = await api.wallet();
		} catch (err) {
			// Un 503 acá significa que el servidor no tiene cadena: no es un
			// error que mostrar, es que la funcionalidad no existe.
			if (err instanceof ApiError && err.status === 503) return;
			this.error = mensajeDeError(err, 'No se pudo leer el estado de la billetera.');
		}
	}

	/**
	 * Conectar: pedir la cuenta, pedir el desafío, firmarlo, mandarlo.
	 *
	 * La dirección se completa del lado del cliente en la segunda línea del
	 * mensaje, que es la que el estándar reserva para eso. El servidor la lee
	 * del texto firmado y la compara con la que recupera de la firma: por eso
	 * la dirección se prueba y no se declara.
	 */
	async conectar(): Promise<void> {
		const eth = proveedor();
		if (!eth) {
			this.error = 'No browser wallet found. Install one to claim distinctions.';
			return;
		}
		this.conectando = true;
		this.error = null;
		try {
			const cuentas = (await eth.request({ method: 'eth_requestAccounts' })) as string[];
			const cuenta = cuentas?.[0];
			if (!cuenta) throw new Error('No account was returned.');

			const desafio = await api.walletChallenge();
			const mensaje = conDireccion(desafio.message, cuenta);
			const firma = (await eth.request({
				method: 'personal_sign',
				params: [mensaje, cuenta]
			})) as string;

			await api.linkWallet({ message: mensaje, signature: firma });
			await this.cargar();
		} catch (err) {
			this.error = mensajeDeError(err, 'No se pudo conectar la billetera.');
		} finally {
			this.conectando = false;
		}
	}

	async desconectar(): Promise<void> {
		this.error = null;
		try {
			await api.unlinkWallet();
			await this.cargar();
		} catch (err) {
			this.error = mensajeDeError(err, 'No se pudo desvincular la billetera.');
		}
	}

	/**
	 * Reclamar: voucher del servidor, transacción de la billetera, confirmación.
	 *
	 * Si la transacción sale y la confirmación falla, el token igual existe: el
	 * txHash se le manda al servidor y, si no pudo verlo, queda pending y se
	 * reintenta al volver a entrar. Por eso el catch de confirmar no borra nada.
	 */
	async reclamar(code: string): Promise<void> {
		const eth = proveedor();
		const estado = this.estado;
		if (!eth || !estado) return;

		this.reclamando = code;
		this.error = null;
		let txHash: string | null = null;
		try {
			const v = await api.voucher(code);

			const { createWalletClient, custom, parseAbi } = await import('viem');
			const cliente = createWalletClient({ account: v.to as `0x${string}`, transport: custom(eth) });

			// Sin `chain` viem no cambia de red sola: si la billetera está en
			// otra, la transacción se manda a la equivocada. Se pide el cambio
			// antes, que es lo único que puede hacerse desde acá.
			await pedirRed(eth, v.chainId);

			txHash = await cliente.writeContract({
				address: v.contract as `0x${string}`,
				abi: parseAbi(MINT_ABI),
				functionName: 'mint',
				args: [v.to as `0x${string}`, v.pieza, BigInt(v.deadline), v.signature as `0x${string}`],
				chain: null
			});
		} catch (err) {
			this.error = mensajeDeError(err, 'No se pudo reclamar la distinción.');
			this.reclamando = null;
			return;
		}

		try {
			await api.confirmMint(code, txHash);
		} catch {
			// El token puede existir igual: la confirmación se reintenta sola.
			this.error = 'Transaction sent. We could not confirm it yet — it will show up shortly.';
		}
		await this.cargar();
		this.reclamando = null;
	}
}

/**
 * Reemplaza la segunda línea del mensaje SIWE con la dirección de la billetera.
 *
 * El servidor emite el mensaje sin saber qué dirección va a firmar —el desafío
 * se pide antes de que la billetera diga quién es—, así que deja ese lugar y lo
 * lee de vuelta. Es exportada porque es la única parte del flujo que se puede
 * probar sin un navegador con billetera.
 */
export function conDireccion(mensaje: string, direccion: string): string {
	const lineas = mensaje.split('\n');
	lineas[1] = direccion.toLowerCase();
	return lineas.join('\n');
}

/** Pide a la billetera que se pare en la red del contrato. */
async function pedirRed(eth: Proveedor, chainId: number): Promise<void> {
	const hex = `0x${chainId.toString(16)}`;
	const actual = (await eth.request({ method: 'eth_chainId' })) as string;
	if (actual?.toLowerCase() === hex) return;
	await eth.request({ method: 'wallet_switchEthereumChain', params: [{ chainId: hex }] });
}

export const wallet = new Wallet();
