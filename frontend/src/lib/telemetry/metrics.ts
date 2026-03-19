import { trace } from '@opentelemetry/api';
import {
    WebTracerProvider,
} from '@opentelemetry/sdk-trace-web';

/**
 * Performance metrics that can be captured
 */
export interface PerformanceMetrics {
    // Navigation timing
    domContentLoaded?: number;
    domComplete?: number;
    loadEventEnd?: number;
    firstContentfulPaint?: number;
    largestContentfulPaint?: number;
    firstMeaningfulPaint?: number;
    timeToFirstByte?: number;
    
    // Core Web Vitals
    cumulativeLayoutShift?: number;
    firstInputDelay?: number;
    interactionToNextPaint?: number;
    
    // Resource timing
    transferSize?: number;
    encodedBodySize?: number;
    decodedBodySize?: number;
    
    // Custom
    ttfb?: number; // Time to First Byte (alias for timeToFirstByte)
    tti?: number; // Time to Interactive
}

/**
 * Initialize performance monitoring with document load and user interaction instrumentation
 * This captures Core Web Vitals and other performance metrics
 */
export function initPerformanceMonitoring(): void {
    // Get the existing provider or create a new one
    const provider = trace.getTracerProvider() as WebTracerProvider | undefined;
    
    if (!provider) {
        console.warn('[telemetry] Performance monitoring requires init() to be called first');
        return;
    }

    // Note: The WebTracerProvider handles instrumentations internally when registered
    // For browser, we rely on the Performance API to capture Core Web Vitals
    
    // Capture Core Web Vitals using Performance API
    captureCoreWebVitals();
    
    // Set up navigation timing observer
    setupNavigationTimingObserver();
    
    // Set up resource timing observer
    setupResourceTimingObserver();

    console.log('[telemetry] Performance monitoring initialized');
}

/**
 * Capture Core Web Vitals and emit them as span events
 */
function captureCoreWebVitals(): void {
    // Use the Paint Timing API for FCP
    if (typeof window !== 'undefined' && window.PerformanceObserver) {
        // First Contentful Paint
        const fcpObserver = new PerformanceObserver((list) => {
            const entries = list.getEntries();
            const fcpEntry = entries.find(
                (entry) => entry.name === 'first-contentful-paint'
            );
            if (fcpEntry) {
                const span = trace.getActiveSpan();
                if (span) {
                    span.setAttribute('webvital.first_contentful_paint', fcpEntry.startTime);
                }
                console.log(`[telemetry] FCP: ${fcpEntry.startTime}ms`);
            }
        });
        
        try {
            fcpObserver.observe({ type: 'paint', buffered: true });
        } catch (e) {
            // FCP observer not supported
        }
        
        // Largest Contentful Paint
        const lcpObserver = new PerformanceObserver((list) => {
            const entries = list.getEntries();
            const lastEntry = entries[entries.length - 1] as PerformancePaintTiming & {
                element?: Element;
            };
            if (lastEntry) {
                const span = trace.getActiveSpan();
                if (span) {
                    span.setAttribute('webvital.largest_contentful_paint', lastEntry.startTime);
                }
                console.log(`[telemetry] LCP: ${lastEntry.startTime}ms`);
            }
        });
        
        try {
            lcpObserver.observe({ type: 'largest-contentful-paint', buffered: true });
        } catch (e) {
            // LCP observer not supported
        }
        
        // First Input Delay
        const fidObserver = new PerformanceObserver((list) => {
            const firstEntry = list.getEntries()[0] as PerformanceEventTiming;
            if (firstEntry) {
                const span = trace.getActiveSpan();
                if (span) {
                    span.setAttribute('webvital.first_input_delay', firstEntry.processingStart - firstEntry.startTime);
                }
                console.log(`[telemetry] FID: ${firstEntry.processingStart - firstEntry.startTime}ms`);
            }
        });
        
        try {
            fidObserver.observe({ type: 'first-input', buffered: true });
        } catch (e) {
            // FID observer not supported
        }
        
        // Cumulative Layout Shift
        const clsObserver = new PerformanceObserver((list) => {
            let clsScore = 0;
            for (const entry of list.getEntries()) {
                const layoutShiftEntry = entry as PerformanceEntry & {
                    hadRecentInput?: boolean;
                    value?: number;
                };
                if (!layoutShiftEntry.hadRecentInput && layoutShiftEntry.value) {
                    clsScore += layoutShiftEntry.value;
                }
            }
            
            const span = trace.getActiveSpan();
            if (span) {
                span.setAttribute('webvital.cumulative_layout_shift', clsScore);
            }
            console.log(`[telemetry] CLS: ${clsScore}`);
        });
        
        try {
            clsObserver.observe({ type: 'layout-shift', buffered: true });
        } catch (e) {
            // CLS observer not supported
        }
    }
}

/**
 * Set up navigation timing observer to capture page load metrics
 */
function setupNavigationTimingObserver(): void {
    if (typeof window !== 'undefined' && window.PerformanceObserver) {
        const navObserver = new PerformanceObserver((list) => {
            const entries = list.getEntries();
            const navEntry = entries[0] as PerformanceNavigationTiming;
            
            if (navEntry) {
                const span = trace.getActiveSpan();
                
                if (span) {
                    // Navigation timing attributes
                    span.setAttribute('navigation.domain_lookup_start', navEntry.domainLookupStart);
                    span.setAttribute('navigation.domain_lookup_end', navEntry.domainLookupEnd);
                    span.setAttribute('navigation.connect_start', navEntry.connectStart);
                    span.setAttribute('navigation.connect_end', navEntry.connectEnd);
                    span.setAttribute('navigation.secure_connection_start', navEntry.secureConnectionStart);
                    span.setAttribute('navigation.request_start', navEntry.requestStart);
                    span.setAttribute('navigation.response_start', navEntry.responseStart);
                    span.setAttribute('navigation.response_end', navEntry.responseEnd);
                    span.setAttribute('navigation.transfer_size', navEntry.transferSize);
                    span.setAttribute('navigation.encoded_body_size', navEntry.encodedBodySize);
                    span.setAttribute('navigation.decoded_body_size', navEntry.decodedBodySize);
                    span.setAttribute('navigation.dom_content_loaded', navEntry.domContentLoadedEventEnd);
                    span.setAttribute('navigation.dom_complete', navEntry.domComplete);
                    span.setAttribute('navigation.load_event_end', navEntry.loadEventEnd);
                    span.setAttribute('navigation.ttfb', navEntry.responseStart - navEntry.requestStart);
                }
                
                console.log(`[telemetry] Navigation timing: TTFB=${navEntry.responseStart - navEntry.requestStart}ms`);
            }
        });
        
        try {
            navObserver.observe({ type: 'navigation', buffered: true });
        } catch (e) {
            // Navigation timing not supported
        }
    }
}

/**
 * Set up resource timing observer to capture resource loading metrics
 */
function setupResourceTimingObserver(): void {
    if (typeof window !== 'undefined' && window.PerformanceObserver) {
        const resourceObserver = new PerformanceObserver((list) => {
            const entries = list.getEntries();
            
            for (const entry of entries) {
                const resourceEntry = entry as PerformanceResourceTiming;
                
                // Only log significant resources
                if (resourceEntry.transferSize > 10000) {
                    console.log(`[telemetry] Resource: ${resourceEntry.name} - ${resourceEntry.duration}ms`);
                }
            }
        });
        
        try {
            resourceObserver.observe({ type: 'resource', buffered: true });
        } catch (e) {
            // Resource timing not supported
        }
    }
}

/**
 * Get current performance metrics from the browser
 * @returns Object containing available performance metrics
 */
export function getPerformanceMetrics(): PerformanceMetrics {
    const metrics: PerformanceMetrics = {};
    
    if (typeof window === 'undefined' || !window.performance) {
        return metrics;
    }
    
    const timing = window.performance.timing;
    const navigation = window.performance.getEntriesByType('navigation')[0] as PerformanceNavigationTiming | undefined;
    
    if (timing) {
        metrics.domContentLoaded = timing.domContentLoadedEventEnd - timing.navigationStart;
        metrics.domComplete = timing.domComplete - timing.navigationStart;
        metrics.loadEventEnd = timing.loadEventEnd - timing.navigationStart;
        metrics.timeToFirstByte = timing.responseStart - timing.navigationStart;
    }
    
    if (navigation) {
        metrics.ttfb = navigation.responseStart - navigation.requestStart;
        metrics.transferSize = navigation.transferSize;
        metrics.encodedBodySize = navigation.encodedBodySize;
        metrics.decodedBodySize = navigation.decodedBodySize;
    }
    
    const paintEntries = window.performance.getEntriesByType('paint');
    const fcpEntry = paintEntries.find((e) => e.name === 'first-contentful-paint');
    if (fcpEntry) {
        metrics.firstContentfulPaint = fcpEntry.startTime;
    }
    
    const lcpEntries = window.performance.getEntriesByType('largest-contentful-paint');
    if (lcpEntries.length > 0) {
        metrics.largestContentfulPaint = lcpEntries[lcpEntries.length - 1].startTime;
    }
    
    return metrics;
}

/**
 * Add a custom performance metric as a span attribute
 * @param name - The name of the metric
 * @param value - The value of the metric
 */
export function addPerformanceMetric(name: string, value: number): void {
    const span = trace.getActiveSpan();
    
    if (span) {
        span.setAttribute(`perf.${name}`, value);
    }
}
