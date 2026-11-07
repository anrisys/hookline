import { describe, expect, it } from 'vitest';
import { parseApiUrl, parseOptional } from './env.ts';

describe('parseApiUrl', () => {
	it.each([
		['unset', undefined, /required/],
		['empty', '', /required/],
		['not a URL', 'http://exa mple.com', /not a valid/],
		// Parses as scheme "localhost:"; the common missing-http:// mistake.
		['scheme-less host:port', 'localhost:8080', /http or https/],
		['relative path', '/api', /not a valid/],
		['non-http scheme', 'ftp://api:21', /http or https/]
	])('rejects %s', (_name, value, message) => {
		expect(() => parseApiUrl(value)).toThrow(message);
	});

	it('does not echo the value in errors', () => {
		expect(() => parseApiUrl('http://user:s3cret@[bad')).toThrow(
			expect.objectContaining({ message: expect.not.stringContaining('s3cret') })
		);
	});

	it.each([
		['http://localhost:8080', 'http://localhost:8080'],
		['http://localhost:8080/', 'http://localhost:8080'],
		['https://api.example.com/v1//', 'https://api.example.com/v1'],
		['http://api:8080', 'http://api:8080']
	])('accepts %s', (value, want) => {
		expect(parseApiUrl(value)).toBe(want);
	});
});

describe('parseOptional', () => {
	it('maps unset and empty to undefined', () => {
		expect(parseOptional(undefined)).toBeUndefined();
		expect(parseOptional('')).toBeUndefined();
	});

	it('keeps a value', () => {
		expect(parseOptional('k')).toBe('k');
	});
});
