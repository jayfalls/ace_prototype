import { describe, it, expect } from 'vitest';
import { cn } from '$lib/utils/cn';

describe('cn', () => {
	it('merges class names', () => {
		const result = cn('foo', 'bar');
		expect(result).toBe('foo bar');
	});

	it('handles conditional classes', () => {
		const isActive = true;
		const result = cn('base', isActive && 'active');
		expect(result).toBe('base active');
	});

	it('handles falsey values', () => {
		const isActive = false;
		const result = cn('base', isActive && 'active');
		expect(result).toBe('base');
	});

	it('handles empty input', () => {
		const result = cn();
		expect(result).toBe('');
	});

	it('handles undefined and null', () => {
		const result = cn('foo', undefined, null, 'bar');
		expect(result).toBe('foo bar');
	});

	it('merges tailwind classes with conflicts', () => {
		const result = cn('text-red-500', 'text-blue-500');
		expect(result).toBe('text-blue-500');
	});
});
