import { describe, it, expect, vi, beforeEach } from 'vitest';
import { authStore } from '$lib/stores/auth.svelte';
import { registerSchema } from '$lib/validation/schemas';

// These tests verify the register page module structure
describe('RegisterPage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('registerSchema should be valid for correct input', () => {
		const result = registerSchema.safeParse({
			username: 'testuser',
			pin: '123456',
			confirmPin: '123456'
		});
		expect(result.success).toBe(true);
	});

	it('registerSchema should reject short pin', () => {
		const result = registerSchema.safeParse({
			username: 'testuser',
			pin: '123',
			confirmPin: '123'
		});
		expect(result.success).toBe(false);
	});

	it('registerSchema should reject mismatched pins', () => {
		const result = registerSchema.safeParse({
			username: 'testuser',
			pin: '123456',
			confirmPin: '654321'
		});
		expect(result.success).toBe(false);
	});

	it('authStore should be importable', () => {
		expect(authStore).toBeDefined();
	});

	it('authStore should have register method', () => {
		expect(typeof authStore.register).toBe('function');
	});
});
