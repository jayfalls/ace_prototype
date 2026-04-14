import { describe, it, expect, vi, beforeEach } from 'vitest';
import { useForm } from '$lib/utils/form.svelte';
import { loginSchema, registerSchema } from '$lib/validation/schemas';
import type { FieldError } from '$lib/api/types';

describe('useForm', () => {
	describe('validate with loginSchema', () => {
		it('sets errors on invalid input', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			const valid = form.validate();
			expect(valid).toBe(false);
			expect(form.errors.username).toBe('Username is required');
			expect(form.errors.pin).toBe('PIN must be at least 4 digits');
		});

		it('clears errors on valid input', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			form.values.username = 'testuser';
			form.values.pin = '123456';

			const valid = form.validate();
			expect(valid).toBe(true);
			expect(form.errors.username).toBeUndefined();
			expect(form.errors.pin).toBeUndefined();
		});
	});

	describe('validateField', () => {
		it('validates single field and marks it as touched', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			form.validateField('username');
			expect(form.touched.username).toBe(true);
			expect(form.errors.username).toBe('Username is required');
		});

		it('clears error when field is valid', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			form.errors.username = 'some error';
			form.values.username = 'testuser';

			form.validateField('username');
			expect(form.errors.username).toBeUndefined();
		});
	});

	describe('reset', () => {
		it('clears all values back to initial', () => {
			const form = useForm({
				initialValues: { username: 'original', pin: '123456' },
				schema: loginSchema,
			});

			form.values.username = 'newuser';
			form.values.pin = '654321';

			form.reset();
			expect(form.values.username).toBe('original');
			expect(form.values.pin).toBe('123456');
		});

		it('clears errors and touched state', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			form.errors.username = 'Username too short';
			form.touched.username = true;

			form.reset();
			expect(Object.keys(form.errors).length).toBe(0);
			expect(form.touched.username).toBeUndefined();
		});

		it('resets isSubmitting to false', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
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
				initialValues: { username: 'testuser', pin: '123456' },
				schema: loginSchema,
			});

			const onSubmit = vi.fn().mockResolvedValue(undefined);
			const handler = form.handleSubmit(onSubmit);

			const event = new Event('submit');
			await handler(event);

			expect(onSubmit).toHaveBeenCalledWith({
				username: 'testuser',
				pin: '123456',
			});
		});

		it('does not call onSubmit when form is invalid', async () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
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
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			const onSubmit = vi.fn().mockResolvedValue(undefined);
			const handler = form.handleSubmit(onSubmit);

			const event = new Event('submit');
			await handler(event);

			expect(form.touched.username).toBe(true);
			expect(form.touched.pin).toBe(true);
		});

		it('sets isSubmitting during submission', async () => {
			const form = useForm({
				initialValues: { username: 'testuser', pin: '123456' },
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
				initialValues: { username: 'testuser', pin: '123456' },
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
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			const apiErrors: FieldError[] = [
				{ field: 'username', message: 'Username already exists' },
				{ field: 'pin', message: 'PIN is incorrect' },
			];

			form.setFieldErrors(apiErrors);
			expect(form.errors.username).toBe('Username already exists');
			expect(form.errors.pin).toBe('PIN is incorrect');
		});

		it('overwrites existing errors', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			form.errors.username = 'Old error';
			const apiErrors: FieldError[] = [
				{ field: 'username', message: 'New error from API' },
			];

			form.setFieldErrors(apiErrors);
			expect(form.errors.username).toBe('New error from API');
		});

		it('handles empty array', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			form.errors.username = 'Some error';
			form.setFieldErrors([]);
			expect(form.errors.username).toBeUndefined();
		});
	});

	describe('isValid', () => {
		it('is true when no errors', () => {
			const form = useForm({
				initialValues: { username: 'testuser', pin: '123456' },
				schema: loginSchema,
			});

			expect(form.isValid).toBe(true);
		});

		it('is false when there are errors', () => {
			const form = useForm({
				initialValues: { username: '', pin: '' },
				schema: loginSchema,
			});

			form.validate();
			expect(form.isValid).toBe(false);
		});
	});

	describe('isDirty', () => {
		it('is false initially', () => {
			const form = useForm({
				initialValues: { username: 'testuser', pin: '123456' },
				schema: loginSchema,
			});

			expect(form.isDirty).toBe(false);
		});

		it('is true when value changes', () => {
			const form = useForm({
				initialValues: { username: 'testuser', pin: '123456' },
				schema: loginSchema,
			});

			form.values.username = 'changeduser';
			expect(form.isDirty).toBe(true);
		});

		it('is false after reset', () => {
			const form = useForm({
				initialValues: { username: 'testuser', pin: '123456' },
				schema: loginSchema,
			});

			form.values.username = 'changeduser';
			form.reset();
			expect(form.isDirty).toBe(false);
		});
	});

	describe('with registerSchema', () => {
		it('validates pin match', () => {
			const form = useForm({
				initialValues: {
					username: 'testuser',
					pin: '123456',
					confirmPin: '654321',
				},
				schema: registerSchema,
			});

			const valid = form.validate();
			expect(valid).toBe(false);
			expect(form.errors.confirmPin).toBe('PINs do not match');
		});

		it('passes when pins match', () => {
			const form = useForm({
				initialValues: {
					username: 'testuser',
					pin: '123456',
					confirmPin: '123456',
				},
				schema: registerSchema,
			});

			const valid = form.validate();
			expect(valid).toBe(true);
		});
	});
});
