import { describe, it, expect, vi, beforeEach } from 'vitest';

// Mock the OpenTelemetry API before importing the module
vi.mock('@opentelemetry/api', () => ({
    trace: {
        getTracerProvider: vi.fn(() => null),
        getActiveSpan: vi.fn(() => null),
    },
}));

import {
    initPerformanceMonitoring,
    getPerformanceMetrics,
    addPerformanceMetric,
} from './metrics';

describe('telemetry/metrics', () => {
    describe('SSR Guard', () => {
        it('should not throw when called in SSR environment', () => {
            // Store original window
            const originalWindow = global.window;
            
            // Simulate SSR environment
            delete (global as Record<string, unknown>).window;
            
            // Should not throw
            expect(() => initPerformanceMonitoring()).not.toThrow();
            
            // Restore window
            global.window = originalWindow as Window & typeof globalThis;
        });
    });

    describe('initPerformanceMonitoring', () => {
        it('should warn when no tracer provider is initialized', () => {
            const consoleWarnSpy = vi.spyOn(console, 'warn').mockImplementation(() => {});
            
            initPerformanceMonitoring();
            
            expect(consoleWarnSpy).toHaveBeenCalledWith(
                '[telemetry] Performance monitoring requires init() to be called first'
            );
            
            consoleWarnSpy.mockRestore();
        });
    });

    describe('getPerformanceMetrics', () => {
        it('should return empty object in SSR environment', () => {
            // Store original window
            const originalWindow = global.window;
            
            // Simulate SSR environment
            delete (global as Record<string, unknown>).window;
            
            const metrics = getPerformanceMetrics();
            expect(metrics).toEqual({});
            
            // Restore window
            global.window = originalWindow as Window & typeof globalThis;
        });

        it('should return performance metrics in browser environment', () => {
            // Mock window with performance API
            const mockTiming = {
                navigationStart: 1000,
                domContentLoadedEventEnd: 2000,
                domComplete: 3000,
                loadEventEnd: 4000,
                responseStart: 1500,
            };
            
            global.window = {
                performance: {
                    timing: mockTiming,
                    getEntriesByType: vi.fn(() => [
                        { name: 'first-contentful-paint', startTime: 500 },
                    ]),
                },
            } as unknown as Window & typeof globalThis;

            const metrics = getPerformanceMetrics();
            
            expect(metrics.domContentLoaded).toBe(1000); // 2000 - 1000
            expect(metrics.domComplete).toBe(2000); // 3000 - 1000
            expect(metrics.loadEventEnd).toBe(3000); // 4000 - 1000
            expect(metrics.firstContentfulPaint).toBe(500);
        });
    });

    describe('addPerformanceMetric', () => {
        it('should not throw when no active span exists', () => {
            expect(() => addPerformanceMetric('test', 123)).not.toThrow();
        });

        it('should handle undefined span gracefully', () => {
            // This test verifies the function works when getActiveSpan returns undefined
            expect(() => addPerformanceMetric('metricName', 42)).not.toThrow();
        });
    });
});
