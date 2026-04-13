import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useForm } from '$lib/utils/form.svelte';
import { loginSchema, registerSchema } from '$lib/validation/schemas';
import type { FieldError } from '$lib/api/types';

describe('useForm', () => {
	describe('validate', () => {
		it('sets errors on invalid input', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			const valid = form.validate();
			expect(valid).toBe(false);
			expect(form.errors.email).toBe('Invalid email format');
			expect(form.errors.password).toBe('Password must be at least 8 characters');
		});

		it('clears errors on valid input', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			form.values.email = 'test@example.com';
			form.values.password = 'password123';

			const valid = form.validate();
			expect(valid).toBe(true);
			expect(form.errors.email).toBeUndefined();
			expect(form.errors.password).toBeUndefined();
		});
	});

	describe('validateField', () => {
		it('validates single field and marks it as touched', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			form.validateField('email');
			expect(form.touched.email).toBe(true);
			expect(form.errors.email).toBe('Invalid email format');
		});

		it('clears error when field is valid', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			form.errors.email = 'some error';
			form.values.email = 'test@example.com';

			form.validateField('email');
			expect(form.errors.email).toBeUndefined();
		});
	});

	describe('reset', () => {
		it('clears all values back to initial', () => {
			const form = useForm({
				initialValues: { email: 'original@example.com', password: 'password123' },
				schema: loginSchema,
			});

			form.values.email = 'new@example.com';
			form.values.password = 'newpassword';

			form.reset();
			expect(form.values.email).toBe('original@example.com');
			expect(form.values.password).toBe('password123');
		});

		it('clears errors and touched state', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			form.errors.email = 'Invalid email format';
			form.touched.email = true;

			form.reset();
			expect(Object.keys(form.errors).length).toBe(0);
			expect(form.touched.email).toBeUndefined();
		});

		it('resets isSubmitting to false', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			form.isSubmitting = true;
			form.reset();
			expect(form.isSubmitting).toBe(false);
		});
	});

	describe('handleSubmit', () => {
		it('calls onSubmit when form is valid', async () => {
			const form = useForm({
				initialValues: { email: 'test@example.com', password: 'password123' },
				schema: loginSchema,
			});

			const onSubmit = vi.fn().mockResolvedValue(undefined);
			const handler = form.handleSubmit(onSubmit);

			const event = new Event('submit');
			await handler(event);

			expect(onSubmit).toHaveBeenCalledWith({
				email: 'test@example.com',
				password: 'password123',
			});
		});

		it('does not call onSubmit when form is invalid', async () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			const onSubmit = vi.fn().mockResolvedValue(undefined);
			const handler = form.handleSubmit(onSubmit);

			const event = new Event('submit');
			await handler(event);

			expect(onSubmit).not.toHaveBeenCalled();
		});

		it('marks all fields as touched on submit', async () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			const onSubmit = vi.fn().mockResolvedValue(undefined);
			const handler = form.handleSubmit(onSubmit);

			const event = new Event('submit');
			await handler(event);

			expect(form.touched.email).toBe(true);
			expect(form.touched.password).toBe(true);
		});

		it('sets isSubmitting during submission', async () => {
			const form = useForm({
				initialValues: { email: 'test@example.com', password: 'password123' },
				schema: loginSchema,
			});

			let checkSubmitting = false;
			const onSubmit = vi.fn().mockImplementation(() => {
				checkSubmitting = form.isSubmitting;
				return Promise.resolve();
			});
			const handler = form.handleSubmit(onSubmit);

			const event = new Event('submit');
			await handler(event);

			expect(checkSubmitting).toBe(true);
			expect(form.isSubmitting).toBe(false);
		});

		it('resets isSubmitting even if submit throws', async () => {
			const form = useForm({
				initialValues: { email: 'test@example.com', password: 'password123' },
				schema: loginSchema,
			});

			const onSubmit = vi.fn().mockRejectedValue(new Error('API error'));
			const handler = form.handleSubmit(onSubmit);

			const event = new Event('submit');
			await expect(handler(event)).rejects.toThrow('API error');

			expect(form.isSubmitting).toBe(false);
		});
	});

	describe('setFieldErrors', () => {
		it('maps API errors to form fields', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			const apiErrors: FieldError[] = [
				{ field: 'email', message: 'Email already exists' },
				{ field: 'password', message: 'Password too weak' },
			];

			form.setFieldErrors(apiErrors);
			expect(form.errors.email).toBe('Email already exists');
			expect(form.errors.password).toBe('Password too weak');
		});

		it('overwrites existing errors', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			form.errors.email = 'Old error';
			const apiErrors: FieldError[] = [
				{ field: 'email', message: 'New error from API' },
			];

			form.setFieldErrors(apiErrors);
			expect(form.errors.email).toBe('New error from API');
		});

		it('handles empty array', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			form.errors.email = 'Some error';
			form.setFieldErrors([]);
			expect(form.errors.email).toBeUndefined();
		});
	});

	describe('isValid', () => {
		it('is true when no errors', () => {
			const form = useForm({
				initialValues: { email: 'test@example.com', password: 'password123' },
				schema: loginSchema,
			});

			expect(form.isValid).toBe(true);
		});

		it('is false when there are errors', () => {
			const form = useForm({
				initialValues: { email: '', password: '' },
				schema: loginSchema,
			});

			form.validate();
			expect(form.isValid).toBe(false);
		});
	});

	describe('isDirty', () => {
		it('is false initially', () => {
			const form = useForm({
				initialValues: { email: 'test@example.com', password: 'password123' },
				schema: loginSchema,
			});

			expect(form.isDirty).toBe(false);
		});

		it('is true when value changes', () => {
			const form = useForm({
				initialValues: { email: 'test@example.com', password: 'password123' },
				schema: loginSchema,
			});

			form.values.email = 'changed@example.com';
			expect(form.isDirty).toBe(true);
		});

		it('is false after reset', () => {
			const form = useForm({
				initialValues: { email: 'test@example.com', password: 'password123' },
				schema: loginSchema,
			});

			form.values.email = 'changed@example.com';
			form.reset();
			expect(form.isDirty).toBe(false);
		});
	});

	describe('with registerSchema', () => {
		it('validates password match', () => {
			const form = useForm({
				initialValues: {
					email: 'test@example.com',
					password: 'password123',
					confirmPassword: 'different',
				},
				schema: registerSchema,
			});

			const valid = form.validate();
			expect(valid).toBe(false);
			expect(form.errors.confirmPassword).toBe('Passwords do not match');
		});

		it('passes when passwords match', () => {
			const form = useForm({
				initialValues: {
					email: 'test@example.com',
					password: 'password123',
					confirmPassword: 'password123',
				},
				schema: registerSchema,
			});

			const valid = form.validate();
			expect(valid).toBe(true);
		});
	});
});
