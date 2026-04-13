import { describe, it, expect } from 'vitest';
import {
	loginSchema,
	registerSchema,
	forgotPasswordSchema,
	resetPasswordSchema,
	suspendUserSchema,
	updateUserRoleSchema,
} from '$lib/validation/schemas';

describe('loginSchema', () => {
	it('passes with valid username and pin', () => {
		const result = loginSchema.safeParse({
			username: 'testuser',
			pin: '123456',
		});
		expect(result.success).toBe(true);
	});

	it('fails with empty username', () => {
		const result = loginSchema.safeParse({
			username: '',
			pin: '123456',
		});
		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error.issues[0].message).toBe('Username is required');
		}
	});

	it('fails with short pin', () => {
		const result = loginSchema.safeParse({
			username: 'testuser',
			pin: '123',
		});
		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error.issues[0].message).toBe('PIN must be at least 4 digits');
		}
	});

	it('fails with non-numeric pin', () => {
		const result = loginSchema.safeParse({
			username: 'testuser',
			pin: 'abcdef',
		});
		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error.issues[0].message).toBe('PIN must contain only digits');
		}
	});

	it('fails with whitespace-only username', () => {
		const result = loginSchema.safeParse({
			username: '   ',
			pin: '123456',
		});
		expect(result.success).toBe(false);
	});
});

describe('registerSchema', () => {
	it('passes with valid inputs', () => {
		const result = registerSchema.safeParse({
			username: 'testuser',
			email: 'test@example.com',
			pin: '123456',
			confirmPin: '123456',
		});
		expect(result.success).toBe(true);
	});

	it('fails when pins do not match', () => {
		const result = registerSchema.safeParse({
			username: 'testuser',
			email: 'test@example.com',
			pin: '123456',
			confirmPin: '654321',
		});
		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error.issues[0].message).toBe('PINs do not match');
			expect(result.error.issues[0].path).toContain('confirmPin');
		}
	});

	it('fails with invalid email', () => {
		const result = registerSchema.safeParse({
			username: 'testuser',
			email: 'notanemail',
			pin: '123456',
			confirmPin: '123456',
		});
		expect(result.success).toBe(false);
	});

	it('fails with short username', () => {
		const result = registerSchema.safeParse({
			username: 'ab',
			email: 'test@example.com',
			pin: '123456',
			confirmPin: '123456',
		});
		expect(result.success).toBe(false);
	});

	it('fails with short pin', () => {
		const result = registerSchema.safeParse({
			username: 'testuser',
			email: 'test@example.com',
			pin: '123',
			confirmPin: '123',
		});
		expect(result.success).toBe(false);
	});

	it('fails with empty confirm pin', () => {
		const result = registerSchema.safeParse({
			username: 'testuser',
			email: 'test@example.com',
			pin: '123456',
			confirmPin: '',
		});
		expect(result.success).toBe(false);
	});
});

describe('forgotPasswordSchema', () => {
	it('passes with valid email', () => {
		const result = forgotPasswordSchema.safeParse({
			email: 'test@example.com',
		});
		expect(result.success).toBe(true);
	});

	it('fails with invalid email', () => {
		const result = forgotPasswordSchema.safeParse({
			email: 'notanemail',
		});
		expect(result.success).toBe(false);
	});

	it('fails with empty email', () => {
		const result = forgotPasswordSchema.safeParse({
			email: '',
		});
		expect(result.success).toBe(false);
	});
});

describe('resetPasswordSchema', () => {
	it('passes with valid inputs', () => {
		const result = resetPasswordSchema.safeParse({
			newPassword: 'password123',
			confirmPassword: 'password123',
		});
		expect(result.success).toBe(true);
	});

	it('fails when passwords do not match', () => {
		const result = resetPasswordSchema.safeParse({
			newPassword: 'password123',
			confirmPassword: 'different',
		});
		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error.issues[0].message).toBe('Passwords do not match');
		}
	});

	it('fails with short new password', () => {
		const result = resetPasswordSchema.safeParse({
			newPassword: 'short',
			confirmPassword: 'short',
		});
		expect(result.success).toBe(false);
	});
});

describe('suspendUserSchema', () => {
	it('passes with no reason', () => {
		const result = suspendUserSchema.safeParse({});
		expect(result.success).toBe(true);
	});

	it('passes with optional reason', () => {
		const result = suspendUserSchema.safeParse({
			reason: 'Violation of terms',
		});
		expect(result.success).toBe(true);
	});

	it('passes with empty reason', () => {
		const result = suspendUserSchema.safeParse({
			reason: '',
		});
		expect(result.success).toBe(true);
	});

	it('fails with reason exceeding 500 characters', () => {
		const result = suspendUserSchema.safeParse({
			reason: 'a'.repeat(501),
		});
		expect(result.success).toBe(false);
		if (!result.success) {
			expect(result.error.issues[0].message).toBe(
				'Reason must be at most 500 characters'
			);
		}
	});

	it('passes with exactly 500 characters', () => {
		const result = suspendUserSchema.safeParse({
			reason: 'a'.repeat(500),
		});
		expect(result.success).toBe(true);
	});
});

describe('updateUserRoleSchema', () => {
	it('passes with admin role', () => {
		const result = updateUserRoleSchema.safeParse({ role: 'admin' });
		expect(result.success).toBe(true);
	});

	it('passes with user role', () => {
		const result = updateUserRoleSchema.safeParse({ role: 'user' });
		expect(result.success).toBe(true);
	});

	it('passes with viewer role', () => {
		const result = updateUserRoleSchema.safeParse({ role: 'viewer' });
		expect(result.success).toBe(true);
	});

	it('fails with invalid role', () => {
		const result = updateUserRoleSchema.safeParse({ role: 'superadmin' });
		expect(result.success).toBe(false);
	});

	it('fails with empty role', () => {
		const result = updateUserRoleSchema.safeParse({ role: '' });
		expect(result.success).toBe(false);
	});
});
