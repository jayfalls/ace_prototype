import type { ZodSchema, ZodError } from 'zod';
import type { FieldError } from '$lib/api/types';

interface UseFormOptions<T extends Record<string, unknown>> {
	initialValues: T;
	schema: ZodSchema<T>;
}

interface UseFormReturn<T extends Record<string, unknown>> {
	values: T;
	errors: Partial<Record<keyof T, string>>;
	touched: Partial<Record<keyof T, boolean>>;
	isSubmitting: boolean;
	isValid: boolean;
	isDirty: boolean;
	validate: () => boolean;
	validateField: (field: keyof T) => void;
	reset: () => void;
	handleSubmit: (onSubmit: (values: T) => Promise<void>) => (e: Event) => Promise<void>;
	setFieldErrors: (fieldErrors: FieldError[]) => void;
}

function clearObject<T extends Record<string, unknown>>(obj: T): void {
	Object.keys(obj).forEach((key) => {
		delete obj[key as keyof T];
	});
}

export function useForm<T extends Record<string, unknown>>(
	options: UseFormOptions<T>
): UseFormReturn<T> {
	const { initialValues, schema } = options;

	const values = $state({ ...initialValues }) as T;
	const errors = $state({}) as Partial<Record<keyof T, string>>;
	const touched = $state({}) as Partial<Record<keyof T, boolean>>;
	const submittingState = $state({ value: false });

	const isValid = $derived(Object.values(errors).every((e) => !e));

	const isDirty = $derived(
		JSON.stringify(values) !== JSON.stringify(initialValues)
	);

	function validate(): boolean {
		const result = schema.safeParse(values);
		if (result.success) {
			clearObject(errors);
			return true;
		}

		clearObject(errors);
		const zodError = result.error as ZodError;
		for (const issue of zodError.issues) {
			const path = issue.path[0] as keyof T;
			if (path && !(path in errors)) {
				errors[path] = issue.message;
			}
		}
		return false;
	}

	function validateField(field: keyof T): void {
		touched[field] = true;
		const result = schema.safeParse(values);
		if (!result.success) {
			const zodError = result.error as ZodError;
			for (const issue of zodError.issues) {
				if (issue.path[0] === field) {
					errors[field] = issue.message;
					return;
				}
			}
		}
		delete errors[field];
	}

	function reset(): void {
		const keys = Object.keys(values) as (keyof T)[];
		for (const key of keys) {
			values[key] = initialValues[key];
		}
		clearObject(errors);
		clearObject(touched);
		submittingState.value = false;
	}

	function handleSubmit(
		onSubmit: (values: T) => Promise<void>
	): (e: Event) => Promise<void> {
		return function (e: Event) {
			e.preventDefault();
			for (const key of Object.keys(values) as (keyof T)[]) {
				touched[key] = true;
			}
			const valid = validate();
			if (!valid) return Promise.resolve();
			submittingState.value = true;
			return onSubmit(values).finally(() => {
				submittingState.value = false;
			});
		};
	}

	function setFieldErrors(fieldErrors: FieldError[]): void {
		clearObject(errors);
		for (const fe of fieldErrors) {
			errors[fe.field as keyof T] = fe.message;
		}
	}

	return {
		get values() { return values; },
		get errors() { return errors; },
		get touched() { return touched; },
		get isSubmitting() { return submittingState.value; },
		set isSubmitting(v: boolean) { submittingState.value = v; },
		get isValid() { return isValid; },
		get isDirty() { return isDirty; },
		validate,
		validateField,
		reset,
		handleSubmit,
		setFieldErrors,
	};
}
