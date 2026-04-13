import { z } from 'zod';

export const loginSchema = z
	.object({
		email: z.string().email('Invalid email format'),
		password: z.string().min(8, 'Password must be at least 8 characters'),
	})
	.refine((data) => data.password.trim().length > 0, {
		message: 'Password cannot be empty or whitespace only',
		path: ['password'],
	});

export const registerSchema = z
	.object({
		email: z.string().email('Invalid email format'),
		password: z.string().min(8, 'Password must be at least 8 characters'),
		confirmPassword: z.string().min(8, 'Password must be at least 8 characters'),
	})
	.refine((data) => data.password === data.confirmPassword, {
		message: 'Passwords do not match',
		path: ['confirmPassword'],
	})
	.refine((data) => data.password.trim().length > 0, {
		message: 'Password cannot be empty or whitespace only',
		path: ['password'],
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
