import { trace, SpanStatusCode, type Span, type SpanOptions } from '@opentelemetry/api';

/**
 * Initialize error tracking for uncaught exceptions and unhandled promise rejections
 * This sets up global error handlers that create spans for errors
 */
export function initErrorTracking(): void {
    // Skip if not in browser environment
    if (typeof window === 'undefined') {
        return;
    }

    const tracer = trace.getTracer('ace-frontend');

    // Track uncaught exceptions
    window.addEventListener('error', (event) => {
        const activeSpan = trace.getActiveSpan();
        
        if (activeSpan) {
            activeSpan.setStatus({
                code: SpanStatusCode.ERROR,
                message: event.message || 'Uncaught error',
            });
            
            activeSpan.recordException({
                name: 'Error',
                message: event.message || 'Uncaught error',
                stack: event.error?.stack || '',
            });
            
            activeSpan.end();
        } else {
            // Create a span for errors outside of any active trace
            const span = tracer.startSpan('error', {
                attributes: {
                    'error.type': 'uncaught_exception',
                    'error.message': event.message || 'Uncaught error',
                    'error.stack': event.error?.stack || '',
                    'location.href': window.location.href,
                    'location.pathname': window.location.pathname,
                },
            } as SpanOptions);
            
            span.setStatus({
                code: SpanStatusCode.ERROR,
                message: event.message || 'Uncaught error',
            });
            
            span.end();
        }
        
        console.error('[telemetry] Uncaught exception:', event.message, event.error);
    });

    // Track unhandled promise rejections
    window.addEventListener('unhandledrejection', (event) => {
        const error = event.reason;
        const errorMessage = error instanceof Error ? error.message : String(error);
        const errorStack = error instanceof Error ? error.stack : '';

        const activeSpan = trace.getActiveSpan();
        
        if (activeSpan) {
            activeSpan.setStatus({
                code: SpanStatusCode.ERROR,
                message: errorMessage || 'Unhandled promise rejection',
            });
            
            activeSpan.recordException({
                name: 'UnhandledPromiseRejection',
                message: errorMessage || 'Unhandled promise rejection',
                stack: errorStack,
            });
            
            activeSpan.end();
        } else {
            // Create a span for unhandled rejections outside of any active trace
            const span = tracer.startSpan('error', {
                attributes: {
                    'error.type': 'unhandled_promise_rejection',
                    'error.message': errorMessage,
                    'error.stack': errorStack,
                    'location.href': window.location.href,
                    'location.pathname': window.location.pathname,
                },
            } as SpanOptions);
            
            span.setStatus({
                code: SpanStatusCode.ERROR,
                message: errorMessage,
            });
            
            span.end();
        }
        
        console.error('[telemetry] Unhandled promise rejection:', errorMessage);
    });

    console.log('[telemetry] Error tracking initialized');
}

/**
 * Create a custom error span for tracking application errors
 * @param error - The error to track
 * @param attributes - Additional attributes to add to the span
 * @returns The created span
 */
export function trackError(
    error: Error | string,
    attributes?: Record<string, string | number | boolean>
): Span {
    const tracer = trace.getTracer('ace-frontend');
    
    const errorMessage = error instanceof Error ? error.message : String(error);
    const errorStack = error instanceof Error ? (error.stack || '') : '';
    
    const spanAttributes: Record<string, string | number | boolean> = {
        'error.type': 'handled_error',
        'error.message': errorMessage,
        'error.stack': errorStack,
    };
    
    // Merge user-provided attributes (filtering out undefined values)
    if (attributes) {
        for (const [key, value] of Object.entries(attributes)) {
            if (value !== undefined) {
                spanAttributes[key] = value;
            }
        }
    }
    
    // Only add location info in browser environment
    if (typeof window !== 'undefined') {
        spanAttributes['location.href'] = window.location.href;
        spanAttributes['location.pathname'] = window.location.pathname;
    }
    
    const span = tracer.startSpan('error', {
        attributes: spanAttributes,
    } as SpanOptions);
    
    span.setStatus({
        code: SpanStatusCode.ERROR,
        message: errorMessage,
    });
    
    if (error instanceof Error) {
        span.recordException(error);
    }
    
    return span;
}

/**
 * End an error span created by trackError
 * @param span - The span to end
 */
export function endErrorSpan(span: Span): void {
    span.end();
}
