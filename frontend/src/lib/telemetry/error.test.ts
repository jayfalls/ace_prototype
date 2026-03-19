import { describe, it, expect, vi, beforeEach, afterEach } from 'vitest';

// Mock the OpenTelemetry API before importing the module
vi.mock('@opentelemetry/api', () => ({
    trace: {
        getTracer: vi.fn(() => ({
            startSpan: vi.fn(() => ({
                setStatus: vi.fn(),
                setAttributes: vi.fn(),
                end: vi.fn(),
                recordException: vi.fn(),
            })),
        })),
        getActiveSpan: vi.fn(() => null),
    },
    SpanStatusCode: {
        ERROR: 2,
        OK: 1,
        UNSET: 0,
    },
}));

import { initErrorTracking, trackError, endErrorSpan } from './error';

describe('telemetry/error', () => {
    describe('SSR Guard', () => {
        it('should not throw when called in SSR environment (window is undefined)', () => {
            // Store original window
            const originalWindow = global.window;
            
            // Simulate SSR environment
            delete (global as Record<string, unknown>).window;
            
            // Should not throw
            expect(() => initErrorTracking()).not.toThrow();
            
            // Restore window
            global.window = originalWindow as Window & typeof globalThis;
        });

        it('should initialize error listeners in browser environment', () => {
            // Mock window with event listeners
            const addEventListenerSpy = vi.fn();
            global.window = {
                addEventListener: addEventListenerSpy,
            } as unknown as Window & typeof globalThis;

            initErrorTracking();

            expect(addEventListenerSpy).toHaveBeenCalledWith('error', expect.any(Function));
            expect(addEventListenerSpy).toHaveBeenCalledWith('unhandledrejection', expect.any(Function));
        });
    });

    describe('trackError', () => {
        beforeEach(() => {
            // Ensure window is defined for these tests
            global.window = {
                location: {
                    href: 'http://localhost:3000/test',
                    pathname: '/test',
                },
            } as unknown as Window & typeof globalThis;
        });

        it('should create a span for a string error', () => {
            const span = trackError('test error message');
            expect(span).toBeDefined();
            expect(span.end).toBeDefined();
        });

        it('should create a span for an Error object', () => {
            const error = new Error('test error');
            const span = trackError(error);
            expect(span).toBeDefined();
            expect(span.end).toBeDefined();
        });

        it('should include additional attributes', () => {
            const span = trackError('test error', {
                'custom.attribute': 'custom-value',
            });
            expect(span).toBeDefined();
        });
    });

    describe('endErrorSpan', () => {
        it('should end the span', () => {
            global.window = {
                location: {
                    href: 'http://localhost:3000/test',
                    pathname: '/test',
                },
            } as unknown as Window & typeof globalThis;
            
            const span = trackError('test error');
            const endSpy = vi.spyOn(span, 'end');
            
            endErrorSpan(span);
            
            expect(endSpy).toHaveBeenCalled();
        });
    });
});
