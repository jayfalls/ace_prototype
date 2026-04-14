import { describe, it, expect, vi, beforeEach } from 'vitest';
import { authStore } from '$lib/stores/auth.svelte';
import { loginSchema } from '$lib/validation/schemas';

// These tests verify the login page module structure
describe('LoginPage', () => {
	beforeEach(() => {
		vi.clearAllMocks();
	});

	it('loginSchema should be valid for correct input', () => {
		const result = loginSchema.safeParse({
			username: 'testuser',
			pin: '123456'
		});
		expect(result.success).toBe(true);
	});

	it('loginSchema should reject empty username', () => {
		const result = loginSchema.safeParse({
			username: '',
			pin: '123456'
		});
		expect(result.success).toBe(false);
	});

	it('loginSchema should reject short pin', () => {
		const result = loginSchema.safeParse({
			username: 'testuser',
			pin: '123'
		});
		expect(result.success).toBe(false);
	});

	it('authStore should be importable', () => {
		expect(authStore).toBeDefined();
	});

	it('authStore should have login method', () => {
		expect(typeof authStore.login).toBe('function');
	});
});
