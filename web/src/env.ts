import { defineEnvVars } from '@sveltejs/kit/env';
import { building } from '$app/env';
import { parseApiUrl, parseOptional } from '#lib/server/env.ts';

// Private (server-only) and read when the server starts, not at build time,
// so one build works against any API.
//
// `vite build` also runs the server once to analyse routes, and that runs
// these schemas. API_URL is only enforced outside the build, so building an
// image needs no runtime config; `node build` still refuses to start without
// a valid API_URL.
export const variables = defineEnvVars({
	API_URL: {
		description: 'Base URL of the hookline API, e.g. http://localhost:8080',
		schema: (value) => (building ? (value ?? '') : parseApiUrl(value))
	},
	API_KEY: {
		description: 'Bearer token sent to the API. Never sent to the browser.',
		schema: parseOptional
	}
});
