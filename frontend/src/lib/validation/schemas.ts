import { z } from 'zod';

export const loginSchema = z
	.object({
		username: z.string().min(1, 'Username is required'),
		pin: z.string().min(4, 'PIN must be at least 4 digits').max(6, 'PIN must be at most 6 digits').regex(/^\d+$/, 'PIN must contain only digits'),
	})
	.refine((data) => data.username.trim().length > 0, {
		message: 'Username cannot be empty or whitespace only',
		path: ['username'],
	});

export const registerSchema = z
	.object({
		username: z.string().min(3, 'Username must be at least 3 characters').max(20, 'Username must be at most 20 characters').regex(/^[a-zA-Z0-9_]+$/, 'Username can only contain letters, numbers, and underscores'),
		pin: z.string().min(4, 'PIN must be at least 4 digits').max(6, 'PIN must be at most 6 digits').regex(/^\d+$/, 'PIN must contain only digits'),
		confirmPin: z.string().min(4, 'PIN must be at least 4 digits').max(6, 'PIN must be at most 6 digits'),
		email: z.string().email('Invalid email format'),
	})
	.refine((data) => data.pin === data.confirmPin, {
		message: 'PINs do not match',
		path: ['confirmPin'],
	})
	.refine((data) => data.username.trim().length > 0, {
		message: 'Username cannot be empty or whitespace only',
		path: ['username'],
	});

export const forgotPasswordSchema = z.object({
	email: z.string().email('Invalid email format'),
});

export const resetPasswordSchema = z
	.object({
		newPassword: z.string().min(8, 'Password must be at least 8 characters'),
		confirmPassword: z.string().min(8, 'Password must be at least 8 characters'),
	})
	.refine((data) => data.newPassword === data.confirmPassword, {
		message: 'Passwords do not match',
		path: ['confirmPassword'],
	})
	.refine((data) => data.newPassword.trim().length > 0, {
		message: 'Password cannot be empty or whitespace only',
		path: ['newPassword'],
	});

export const suspendUserSchema = z.object({
	reason: z.string().max(500, 'Reason must be at most 500 characters').optional(),
});

export const updateUserRoleSchema = z.object({
	role: z.enum(['admin', 'user', 'viewer']),
});

export type LoginInput = z.infer<typeof loginSchema>;
export type RegisterInput = z.infer<typeof registerSchema>;
export type ForgotPasswordInput = z.infer<typeof forgotPasswordSchema>;
export type ResetPasswordInput = z.infer<typeof resetPasswordSchema>;
export type SuspendUserInput = z.infer<typeof suspendUserSchema>;
export type UpdateUserRoleInput = z.infer<typeof updateUserRoleSchema>;
