import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock the OpenTelemetry API before importing the module
vi.mock('@opentelemetry/api', () => ({
    trace: {
        getTracer: vi.fn(() => ({
            startSpan: vi.fn(() => ({
                spanContext: vi.fn(() => ({ traceId: 'mock-trace-id-12345678901234567890123456789012' })),
                setStatus: vi.fn(),
                setAttributes: vi.fn(),
                end: vi.fn(),
                recordException: vi.fn(),
            })),
            startActiveSpan: vi.fn(async (name, fn) => {
                const mockSpan = {
                    spanContext: vi.fn(() => ({ traceId: 'mock-trace-id-12345678901234567890123456789012' })),
                    setStatus: vi.fn(),
                    setAttributes: vi.fn(),
                    end: vi.fn(),
                    recordException: vi.fn(),
                };
                return fn(mockSpan);
            }),
        })),
        getActiveSpan: vi.fn(() => null),
        getTracerProvider: vi.fn(() => null),
        setGlobalTracerProvider: vi.fn(),
    },
    context: {
        active: vi.fn(() => ({})),
        with: vi.fn((ctx, fn) => fn()),
    },
    propagation: {
        inject: vi.fn(),
        extract: vi.fn(() => ({})),
    },
    ROOT_CONTEXT: {},
    SpanStatusCode: {
        ERROR: 2,
        OK: 1,
        UNSET: 0,
    },
}));

import { getTraceId, setGlobalTraceId, startSpan, withSpan, shutdown } from './trace';

describe('telemetry/trace', () => {
    describe('getTraceId', () => {
        it('should return null when no active span exists and no global trace id is set', () => {
            const traceId = getTraceId();
            expect(traceId).toBeNull();
        });

        it('should return the global trace id when set', () => {
            setGlobalTraceId('test-trace-id');
            const traceId = getTraceId();
            expect(traceId).toBe('test-trace-id');
        });
    });

    describe('setGlobalTraceId', () => {
        it('should set the global trace id', () => {
            setGlobalTraceId('new-trace-id');
            expect(getTraceId()).toBe('new-trace-id');
        });
    });

    describe('startSpan', () => {
        it('should create a span with the given name', () => {
            const span = startSpan('test-span');
            expect(span).toBeDefined();
        });

        it('should create a span with attributes', () => {
            const span = startSpan('test-span', {
                attributes: { 'test.key': 'test-value' },
            });
            expect(span).toBeDefined();
        });
    });

    describe('withSpan', () => {
        it('should execute the callback and return the result', async () => {
            const result = await withSpan('test-span', async () => {
                return 'success';
            });
            expect(result).toBe('success');
        });

        it('should pass through errors from the callback', async () => {
            await expect(
                withSpan('test-span', async () => {
                    throw new Error('Test error');
                })
            ).rejects.toThrow('Test error');
        });
    });

    describe('shutdown', () => {
        it('should complete without error', async () => {
            await expect(shutdown()).resolves.toBeUndefined();
        });
    });
});
