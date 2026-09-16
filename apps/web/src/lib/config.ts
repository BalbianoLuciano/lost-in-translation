import * as publicEnv from '$env/static/public';

// $env/static/public sólo trae las variables definidas al buildear; por eso se
// lee como diccionario y no con imports con nombre.
const env = publicEnv as Record<string, string | undefined>;

export const API_URL = (env.PUBLIC_API_URL ?? 'http://localhost:8080').replace(/\/$/, '');

export const firebaseConfig = {
	apiKey: env.PUBLIC_FIREBASE_API_KEY ?? '',
	authDomain: env.PUBLIC_FIREBASE_AUTH_DOMAIN ?? '',
	projectId: env.PUBLIC_FIREBASE_PROJECT_ID ?? '',
	appId: env.PUBLIC_FIREBASE_APP_ID ?? ''
};

export const firebaseConfigured = Object.values(firebaseConfig).every(Boolean);

/** Login falso para desarrollo local sin proyecto de Firebase. La API lo acepta con AUTH_MODE=dev. */
export const devAuthEnabled = !firebaseConfigured && import.meta.env.DEV;
