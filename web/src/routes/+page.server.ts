import { apiFetch } from '#lib/server/client.ts';
import { getHealth } from '#lib/server/health.ts';
import type { PageServerLoad } from './$types';

// Server-only load: the browser never talks to the API or sees the key.
// Runs on every request (no prerendering), so a reload shows the API's
// current state.
export const prerender = false;

export const load: PageServerLoad = async ({ fetch }) => {
	return { api: await getHealth((path) => apiFetch(fetch, path)) };
};
