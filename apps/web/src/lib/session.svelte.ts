import { firebaseConfig, firebaseConfigured, devAuthEnabled } from '$lib/config';
import type { Auth } from 'firebase/auth';

export type SessionUser = {
	uid: string;
	email: string;
	name: string;
};

const DEV_USER_KEY = 'lit-dev-user';

class Session {
	user = $state<SessionUser | null>(null);
	ready = $state(false);
	error = $state<string | null>(null);

	#auth: Auth | null = null;

	get mode(): 'firebase' | 'dev' | 'unconfigured' {
		if (firebaseConfigured) return 'firebase';
		return devAuthEnabled ? 'dev' : 'unconfigured';
	}

	async init(): Promise<void> {
		if (this.mode === 'firebase') {
			const [{ initializeApp }, { getAuth, onAuthStateChanged }] = await Promise.all([
				import('firebase/app'),
				import('firebase/auth')
			]);
			this.#auth = getAuth(initializeApp(firebaseConfig));
			onAuthStateChanged(this.#auth, (u) => {
				this.user = u ? { uid: u.uid, email: u.email ?? '', name: u.displayName ?? '' } : null;
				this.ready = true;
			});
			return;
		}
		if (this.mode === 'dev') {
			try {
				const raw = localStorage.getItem(DEV_USER_KEY);
				this.user = raw ? (JSON.parse(raw) as SessionUser) : null;
			} catch {
				this.user = null;
			}
		}
		this.ready = true;
	}

	async signIn(): Promise<void> {
		this.error = null;
		if (this.mode === 'dev') {
			this.user = { uid: 'dev-lucho', email: 'lucho@dev.local', name: 'Lucho (dev)' };
			localStorage.setItem(DEV_USER_KEY, JSON.stringify(this.user));
			return;
		}
		if (!this.#auth) return;
		const { GoogleAuthProvider, signInWithPopup, signInWithRedirect } = await import('firebase/auth');
		const provider = new GoogleAuthProvider();
		try {
			await signInWithPopup(this.#auth, provider);
		} catch (err) {
			const code = (err as { code?: string }).code ?? '';
			// En la PWA instalada o con popups bloqueados, se cae a redirect.
			if (code === 'auth/popup-blocked' || code === 'auth/operation-not-supported-in-this-environment') {
				await signInWithRedirect(this.#auth, provider);
				return;
			}
			if (code !== 'auth/popup-closed-by-user' && code !== 'auth/cancelled-popup-request') {
				this.error = 'No se pudo entrar con Google. Probá de nuevo.';
			}
		}
	}

	async signOut(): Promise<void> {
		if (this.mode === 'dev') {
			localStorage.removeItem(DEV_USER_KEY);
			this.user = null;
			return;
		}
		if (!this.#auth) return;
		const { signOut } = await import('firebase/auth');
		await signOut(this.#auth);
	}

	/** Token para la API: ID token de Firebase o el token falso de desarrollo. */
	async token(): Promise<string | null> {
		if (!this.user) return null;
		if (this.mode === 'dev') return `dev:${this.user.uid}:${this.user.email}`;
		return (await this.#auth?.currentUser?.getIdToken()) ?? null;
	}
}

export const session = new Session();
