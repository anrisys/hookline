import { json } from '@sveltejs/kit';
import type { RequestHandler } from './$types';

// Liveness for the container healthcheck: the Node server is up and routing.
// Deliberately independent of the API, so an API outage shows as "API: down"
// on the page rather than taking the web container down with it.
export const prerender = false;

export const GET: RequestHandler = () => json({ status: 'ok' });
