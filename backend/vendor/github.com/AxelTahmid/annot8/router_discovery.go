package annot8

import (
	"fmt"
	"net/http"
	"reflect"
	"runtime"
	"strings"

	"github.com/go-chi/chi/v5"
)

// RouteInfo holds metadata about each registered route
// including HTTP method, path pattern, handler name, and function.
type RouteInfo struct {
	Method      string
	Pattern     string
	HandlerName string
	HandlerFunc http.HandlerFunc
	Middlewares []func(http.Handler) http.Handler
}

// RouteDiscoveryError represents an error that occurred during route discovery.
type RouteDiscoveryError struct {
	Operation string
	Err       error
}

func (e *RouteDiscoveryError) Error() string {
	return fmt.Sprintf("route discovery %s: %v", e.Operation, e.Err)
}

func (e *RouteDiscoveryError) Unwrap() error {
	return e.Err
}

// InspectRoutes walks a Chi router and returns a list of RouteInfo.
// Returns an error if the router traversal fails or if route analysis encounters issues.
func InspectRoutes(r chi.Router) ([]RouteInfo, error) {
	if r == nil {
		return nil, &RouteDiscoveryError{
			Operation: "inspect",
			Err:       fmt.Errorf("router cannot be nil"),
		}
	}

	var routes []RouteInfo
	err := chi.Walk(
		r,
		func(method string, route string, handler http.Handler, middlewares ...func(http.Handler) http.Handler) error {
			// Attempt to extract http.HandlerFunc
			var hf http.HandlerFunc
			switch h := handler.(type) {
			case http.HandlerFunc:
				hf = h
			default:
				// wrap other handlers
				hf = h.ServeHTTP
			}
			name := runtime.FuncForPC(reflect.ValueOf(hf).Pointer()).Name()
			routes = append(routes, RouteInfo{
				Method:      method,
				Pattern:     route,
				HandlerName: name,
				HandlerFunc: hf,
				Middlewares: middlewares,
			})
			return nil
		},
	)

	if err != nil {
		return nil, &RouteDiscoveryError{
			Operation: "walk",
			Err:       err,
		}
	}

	return routes, nil
}

// DiscoverRoutes returns only non-internal routes for OpenAPI spec assembly.
// This function filters out routes that are part of the OpenAPI tooling itself
// (such as /swagger and /annot8 endpoints) to avoid circular references in the specification.
func DiscoverRoutes(r chi.Router) ([]RouteInfo, error) {
	// Retrieve all routes via InspectRoutes
	infos, err := InspectRoutes(r)
	if err != nil {
		return nil, err
	}
	var filtered []RouteInfo
	for _, ri := range infos {
		// Skip documentation/internal routes (swagger, annot8, annot8)
		if strings.Contains(ri.Pattern, "/swagger") || strings.Contains(ri.Pattern, "/annot8") || strings.Contains(ri.Pattern, "/annot8") {
			continue
		}
		filtered = append(filtered, ri)
	}
	return filtered, nil
}
