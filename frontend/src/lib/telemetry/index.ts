/**
 * Frontend Telemetry Module
 * 
 * This module provides OpenTelemetry integration for the SvelteKit frontend,
 * enabling distributed tracing, error tracking, and performance monitoring.
 * 
 * @package @ace/frontend
 */

import { init, getTraceId, setGlobalTraceId, startSpan, withSpan, shutdown, getCurrentContext, withContext, injectContext, extractContext } from './trace';
import { initErrorTracking, trackError, endErrorSpan } from './error';
import { initPerformanceMonitoring, getPerformanceMetrics, addPerformanceMetric } from './metrics';

/**
 * Initialize the telemetry module with OpenTelemetry
 * 
 * This function sets up:
 * - OpenTelemetry tracing with W3C Trace Context propagation
 * - Error tracking for uncaught exceptions and unhandled promise rejections
 * - Performance monitoring for Core Web Vitals and page load metrics
 * 
 * @param serviceName - The name of the service (e.g., "ace-frontend")
 * @param otelCollectorUrl - The URL of the OTel Collector (e.g., "http://localhost:4318")
 * 
 * @example
 * ```typescript
 * import { initTelemetry } from '$lib/telemetry';
 * 
 * // In your app's initialization code (e.g., +layout.svelte)
 * initTelemetry('ace-frontend', 'http://localhost:4318');
 * ```
 */
export function initTelemetry(serviceName: string, otelCollectorUrl: string): void {
    // Initialize tracing first
    init(serviceName, otelCollectorUrl);
    
    // Then initialize error tracking
    initErrorTracking();
    
    // Finally, initialize performance monitoring
    initPerformanceMonitoring();
    
    console.log(`[telemetry] Initialized complete telemetry for ${serviceName}`);
}

// Re-export all trace functions
export {
    getTraceId,
    setGlobalTraceId,
    startSpan,
    withSpan,
    shutdown,
    getCurrentContext,
    withContext,
    injectContext,
    extractContext,
};

// Re-export all error tracking functions
export {
    initErrorTracking,
    trackError,
    endErrorSpan,
};

// Re-export all performance monitoring functions
export {
    initPerformanceMonitoring,
    getPerformanceMetrics,
    addPerformanceMetric,
};
