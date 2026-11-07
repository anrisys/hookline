// Validators for the variables declared in src/env.ts. They run once when the
// server starts; a thrown error stops startup with that message.

/** Requires an absolute http(s) URL and returns it without a trailing slash. */
export function parseApiUrl(value: string | undefined): string {
	if (!value) throw new Error('API_URL is required, e.g. http://localhost:8080');
	let url: URL;
	try {
		url = new URL(value);
	} catch {
		// The value is not echoed: a URL can carry credentials.
		throw new Error('API_URL is not a valid absolute URL');
	}
	if (url.protocol !== 'http:' && url.protocol !== 'https:') {
		throw new Error('API_URL must use http or https');
	}
	return url.href.replace(/\/+$/, '');
}

/** Treats unset and empty the same: both mean "not configured". */
export function parseOptional(value: string | undefined): string | undefined {
	return value ? value : undefined;
}
