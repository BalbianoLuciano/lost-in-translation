import { expect, test, type Page } from '@playwright/test';
import { drills, skillNames } from './content';

// La transcripción de mentira está fija en playwright.config.ts y dice "she".
// El primer drill que ofrece la app tiene que ser uno que espere "she", o el
// test estaría probando otra cosa.
const primero = drills[0];

/**
 * Graba, corta y pasa al siguiente hasta llegar a un drill que espere "he".
 *
 * No se puede ir por índice: al responder bien, ese tema deja de ser el más
 * flojo y la app pasa a otro. La secuencia la decide el motor, no el archivo.
 */
async function avanzarHastaUnDrillQueEspera(page: Page, pronombre: string) {
	const porConsigna = new Map(drills.map((d) => [d.prompt_en, d]));
	for (let i = 0; i < drills.length; i++) {
		const consigna = (await page.locator('.consigna').textContent())?.trim() ?? '';
		if (porConsigna.get(consigna)?.expect.includes(pronombre)) return;
		await page.getByRole('button', { name: 'Grabar' }).click();
		await page.waitForTimeout(400);
		await page.getByRole('button', { name: /Parar la grabación/ }).click();
		await expect(page.locator('.veredicto')).toBeVisible({ timeout: 30000 });
		await page.getByRole('button', { name: /Next drill/ }).click();
	}
	throw new Error(`ningún drill espera "${pronombre}": este test no tendría qué probar`);
}

/**
 * Micrófono y grabador de mentira, dentro de la página.
 *
 * El micrófono falso de Chrome no existe en el runner de CI, así que el test se
 * quedaba esperando el permiso. Simulando MediaRecorder se prueba lo que importa
 * —el flujo de la pantalla— y además el resultado es siempre el mismo.
 */
function fakeMic(page: Page) {
	return page.addInitScript(() => {
		class FakeRecorder {
			state = 'inactive';
			mimeType: string;
			ondataavailable: ((e: { data: Blob }) => void) | null = null;
			onstop: (() => void) | null = null;

			constructor(_stream: unknown, options?: { mimeType?: string }) {
				this.mimeType = options?.mimeType ?? 'audio/webm';
			}
			static isTypeSupported() {
				return true;
			}
			start() {
				this.state = 'recording';
			}
			stop() {
				this.state = 'inactive';
				this.ondataavailable?.({ data: new Blob([new Uint8Array(2048)], { type: this.mimeType }) });
				this.onstop?.();
			}
		}
		Object.defineProperty(navigator, 'mediaDevices', {
			configurable: true,
			value: { getUserMedia: async () => ({ getTracks: () => [{ stop() {} }] }) }
		});
		(window as unknown as { MediaRecorder: unknown }).MediaRecorder = FakeRecorder;
	});
}

function freshUser(page: Page, tag: string) {
	const uid = `e2e-${tag}-${Date.now()}-${Math.floor(Math.random() * 1e6)}`;
	return page.addInitScript(
		(user) => localStorage.setItem('lit-dev-user', JSON.stringify(user)),
		{ uid, email: `${uid}@dev.local`, name: 'E2E' }
	);
}

test('un drill oral se graba, se transcribe y se corrige', async ({ page }, testInfo) => {
	await fakeMic(page);
	await freshUser(page, `speak-${testInfo.project.name}`);
	await page.goto('/speaking');

	// El primer drill sale del banco, no de una constante: si el contenido cambia
	// el orden, este test tiene que seguir apuntando al drill que la app ofrece.
	expect(
		primero.expect,
		'el primer drill tiene que esperar "she": es lo que dice la transcripción de mentira de playwright.config.ts'
	).toContain('she');
	await expect(page.getByText(skillNames[primero.skill])).toBeVisible();
	await expect(page.locator('.consigna')).toContainText(primero.prompt_en);

	// La pista está a pedido, como las explicaciones en castellano
	await page.getByRole('button', { name: /Dame una pista/ }).click();
	await expect(page.locator('.pista')).toBeVisible();

	// Grabar, esperar un momento y cortar
	await page.getByRole('button', { name: 'Grabar' }).click();
	await expect(page.getByRole('button', { name: /Parar la grabación/ })).toBeVisible();
	await page.waitForTimeout(400);
	await page.getByRole('button', { name: /Parar la grabación/ }).click();

	// La transcripción de mentira dice "she", que es lo que el drill pedía
	const veredicto = page.locator('.veredicto');
	await expect(veredicto).toBeVisible({ timeout: 30000 });
	await expect(veredicto).toHaveText('Correct.');
	await expect(page.locator('.transcripcion')).toContainText('She found the bug');
	await expect(page.getByText(/\+5 colada/)).toBeVisible();

	// Y sigue con otro, que ya no es el mismo
	await page.getByRole('button', { name: /Next drill/ }).click();
	await expect(page.locator('.consigna')).not.toContainText(primero.prompt_en);
	const siguiente = (await page.locator('.consigna').textContent())?.trim() ?? '';
	expect(drills.map((d) => d.prompt_en)).toContain(siguiente);
});

test('el error de género se marca en la transcripción', async ({ page }, testInfo) => {
	await fakeMic(page);
	await freshUser(page, `speakbad-${testInfo.project.name}`);
	// Hay que llegar a un drill que espere "he", porque la transcripción de
	// mentira dice "she": ahí es donde se ve el error marcado.
	await page.goto('/speaking');
	await avanzarHastaUnDrillQueEspera(page, 'he');
	await page.getByRole('button', { name: 'Grabar' }).click();
	await page.waitForTimeout(400);
	await page.getByRole('button', { name: /Parar la grabación/ }).click();

	const veredicto = page.locator('.veredicto');
	await expect(veredicto).toBeVisible({ timeout: 30000 });
	await expect(veredicto).toContainText('she');
	await expect(page.locator('.transcripcion .mal').first()).toBeVisible();
});
