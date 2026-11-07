// The API client wired to this server's environment. Pure logic lives in
// api.ts so it can be tested without env.
import { API_KEY, API_URL } from '$app/env/private';
import { createApiFetch } from './api.ts';

export const apiFetch = createApiFetch({ baseUrl: API_URL, apiKey: API_KEY });
