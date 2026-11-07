// Calls to the hookline API. Lives under server/ so SvelteKit refuses to
// bundle it (and the API key) into browser code.

export type Fetch = typeof fetch;

export type ApiConfig = {
	/** Base URL without a trailing slash (parseApiUrl guarantees this). */
	baseUrl: string;
	apiKey?: string;
	/** Bounds the whole call, body included. */
	timeoutMs?: number;
};

/** Longer than the API's own 1s health ping, so a 503 arrives as a 503. */
export const DEFAULT_TIMEOUT_MS = 2000;

export type ApiFetch = (fetchFn: Fetch, path: string, init?: RequestInit) => Promise<Response>;

/**
 * Returns a fetch bound to the API: prefixes the base URL, adds the bearer
 * token when one is configured, and aborts after the timeout. Callers pass
 * SvelteKit's event.fetch so request-scoped hooks (tracing in #20) apply.
 */
export function createApiFetch(cfg: ApiConfig): ApiFetch {
	const timeoutMs = cfg.timeoutMs ?? DEFAULT_TIMEOUT_MS;
	return (fetchFn, path, init = {}) => {
		const headers = new Headers(init.headers);
		if (cfg.apiKey) headers.set('Authorization', `Bearer ${cfg.apiKey}`);
		const timeout = AbortSignal.timeout(timeoutMs);
		const signal = init.signal ? AbortSignal.any([init.signal, timeout]) : timeout;
		const url = cfg.baseUrl + (path.startsWith('/') ? path : `/${path}`);
		return fetchFn(url, { ...init, headers, signal });
	};
}
