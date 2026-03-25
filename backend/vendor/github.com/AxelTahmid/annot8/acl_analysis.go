package annot8

import (
	"go/ast"
	"go/token"
	"net/http"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
)

const (
	middlewareImportPath = "middleware"
	aclImportPath        = "acl"
)

// resolveACLPermissions determines ACL requirements for a handler.
func (g *Generator) resolveACLPermissions(
	route, method string,
	handlerInfo *HandlerInfo,
	middlewares []func(http.Handler) http.Handler,
) []string {
	if perms := g.extractPermissionsFromSource(handlerInfo); len(perms) > 0 {
		return perms
	}

	if inferred := inferPermissionFromRoute(route, method, middlewares); inferred != "" {
		return []string{inferred}
	}

	return extractACLPermissions(middlewares)
}

// extractPermissionsFromSource walks router definitions to recover ACL slugs.
func (g *Generator) extractPermissionsFromSource(handlerInfo *HandlerInfo) []string {
	if handlerInfo == nil || handlerInfo.File == "" || g.schemaGen == nil {
		return nil
	}

	ti := g.schemaGen.typeIndex
	if ti == nil {
		return nil
	}

	methodName := getMethodNameFromUnique(handlerInfo.FunctionName)
	if methodName == "" {
		return nil
	}

	methodDecl := findMethodDecl(ti, handlerInfo.File, methodName)
	if methodDecl == nil {
		return nil
	}

	receiver := receiverTypeName(methodDecl)
	if receiver == "" {
		return nil
	}

	routesDecl, routesFile := findRoutesDecl(ti, filepath.Dir(handlerInfo.File), receiver)
	if routesDecl == nil || routesDecl.Body == nil {
		return nil
	}

	slugMap := g.loadACLSlugMap()
	if len(slugMap) == 0 {
		return nil
	}

	middlewareAliases, aclAliases := importAliases(routesFile)
	perms := collectRoutePermissionSlugs(routesDecl, methodName, slugMap, middlewareAliases, aclAliases)
	return perms
}

func (g *Generator) loadACLSlugMap() map[string]string {
	g.aclSlugOnce.Do(func() {
		g.aclSlugMap = buildACLSlugMap(g.schemaGen.typeIndex)
	})
	return g.aclSlugMap
}

func buildACLSlugMap(ti *TypeIndex) map[string]string {
	result := make(map[string]string)
	if ti == nil {
		return result
	}

	targetSuffix := "pkg/acl/slug.go"
	for path, file := range ti.files {
		if !strings.HasSuffix(path, targetSuffix) {
			continue
		}
		for _, decl := range file.Decls {
			gd, ok := decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.CONST {
				continue
			}
			for _, spec := range gd.Specs {
				vs, ok := spec.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for i, name := range vs.Names {
					if name == nil {
						continue
					}
					var expr ast.Expr
					switch {
					case len(vs.Values) > i:
						expr = vs.Values[i]
					case len(vs.Values) == 1:
						expr = vs.Values[0]
					default:
						continue
					}
					if slug := basicLitValue(expr); slug != "" {
						result[name.Name] = slug
					}
				}
			}
		}
		break
	}
	return result
}

func collectRoutePermissionSlugs(
	routesDecl *ast.FuncDecl,
	targetMethod string,
	slugMap map[string]string,
	middlewareAliases, aclAliases []string,
) []string {
	if routesDecl == nil || routesDecl.Body == nil {
		return nil
	}

	var perms []string
	ast.Inspect(routesDecl.Body, func(n ast.Node) bool {
		call, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil || !isHTTPVerb(selector.Sel.Name) {
			return true
		}
		if !handlerMatchesTarget(call, targetMethod) {
			return true
		}
		mwExprs := collectMiddlewareExpressions(selector.X)
		perms = append(perms, extractSlugsFromMiddleware(mwExprs, slugMap, middlewareAliases, aclAliases)...)
		return true
	})

	return uniqueStrings(perms)
}

func handlerMatchesTarget(call *ast.CallExpr, target string) bool {
	if call == nil || len(call.Args) == 0 {
		return false
	}
	return selectorMatchesMethod(call.Args[len(call.Args)-1], target)
}

func selectorMatchesMethod(expr ast.Expr, target string) bool {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		return v.Sel != nil && v.Sel.Name == target
	case *ast.Ident:
		return v.Name == target
	case *ast.CallExpr:
		if len(v.Args) == 0 {
			return false
		}
		return selectorMatchesMethod(v.Args[0], target)
	default:
		return false
	}
}

func isHTTPVerb(name string) bool {
	switch name {
	case "Get", "Post", "Put", "Patch", "Delete", "Options", "Head":
		return true
	default:
		return false
	}
}

func collectMiddlewareExpressions(expr ast.Expr) []ast.Expr {
	var result []ast.Expr
	current := expr
	for {
		call, ok := current.(*ast.CallExpr)
		if !ok {
			break
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil {
			break
		}
		switch selector.Sel.Name {
		case "With", "Use":
			result = append(result, call.Args...)
		}
		current = selector.X
	}
	return result
}

func extractSlugsFromMiddleware(
	exprs []ast.Expr,
	slugMap map[string]string,
	middlewareAliases, aclAliases []string,
) []string {
	mwSet := make(map[string]struct{}, len(middlewareAliases))
	for _, alias := range middlewareAliases {
		mwSet[alias] = struct{}{}
	}

	var perms []string
	for _, expr := range exprs {
		call, ok := expr.(*ast.CallExpr)
		if !ok {
			continue
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || selector.Sel == nil {
			continue
		}
		xIdent, ok := selector.X.(*ast.Ident)
		if !ok {
			continue
		}
		if _, ok := mwSet[xIdent.Name]; !ok {
			continue
		}

		switch selector.Sel.Name {
		case "Can":
			if len(call.Args) != 1 {
				continue
			}
			if slug := slugFromExpr(call.Args[0], slugMap, aclAliases); slug != "" {
				perms = append(perms, slug)
			}
		case "Any":
			slugs := gatherSlugs(call.Args, slugMap, aclAliases)
			if len(slugs) > 0 {
				perms = append(perms, "any("+strings.Join(slugs, ", ")+")")
			}
		case "Must":
			slugs := gatherSlugs(call.Args, slugMap, aclAliases)
			if len(slugs) > 0 {
				perms = append(perms, "all("+strings.Join(slugs, ", ")+")")
			}
		}
	}
	return perms
}

func gatherSlugs(args []ast.Expr, slugMap map[string]string, aclAliases []string) []string {
	var slugs []string
	for _, arg := range args {
		if slug := slugFromExpr(arg, slugMap, aclAliases); slug != "" {
			slugs = append(slugs, slug)
		}
	}
	return slugs
}

func slugFromExpr(expr ast.Expr, slugMap map[string]string, aclAliases []string) string {
	switch v := expr.(type) {
	case *ast.SelectorExpr:
		if ident, ok := v.X.(*ast.Ident); ok {
			if aliasMatches(ident.Name, defaultedAliases(aclAliases, "acl")) {
				if slug, ok := slugMap[v.Sel.Name]; ok {
					return slug
				}
				return strings.ToLower(v.Sel.Name)
			}
		}
	case *ast.BasicLit:
		if v.Kind == token.STRING {
			return strings.Trim(v.Value, `"`)
		}
	}
	return ""
}

func basicLitValue(expr ast.Expr) string {
	if bl, ok := expr.(*ast.BasicLit); ok && bl.Kind == token.STRING {
		return strings.Trim(bl.Value, `"`)
	}
	return ""
}

func getMethodNameFromUnique(unique string) string {
	if unique == "" {
		return ""
	}
	parts := strings.Split(unique, ".")
	return parts[len(parts)-1]
}

func findMethodDecl(ti *TypeIndex, filePath, methodName string) *ast.FuncDecl {
	if ti == nil {
		return nil
	}
	file := ti.LookupFile(filePath)
	if file == nil {
		return nil
	}
	for _, decl := range file.Decls {
		if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name != nil && fd.Name.Name == methodName && fd.Recv != nil {
			return fd
		}
	}
	return nil
}

func receiverTypeName(fd *ast.FuncDecl) string {
	if fd == nil || fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	switch recv := fd.Recv.List[0].Type.(type) {
	case *ast.StarExpr:
		if ident, ok := recv.X.(*ast.Ident); ok {
			return ident.Name
		}
	case *ast.Ident:
		return recv.Name
	}
	return ""
}

func findRoutesDecl(ti *TypeIndex, dir, receiver string) (*ast.FuncDecl, *ast.File) {
	if ti == nil {
		return nil, nil
	}
	normalizedDir := filepath.ToSlash(dir)
	for path, file := range ti.files {
		// Both path and normalizedDir are forward-slashed for comparison, using EqualFold for Windows casing
		if !strings.EqualFold(filepath.ToSlash(filepath.Dir(path)), normalizedDir) {
			continue
		}
		for _, decl := range file.Decls {
			if fd, ok := decl.(*ast.FuncDecl); ok && fd.Name != nil && fd.Name.Name == "Routes" &&
				receiverMatches(fd, receiver) {
				return fd, file
			}
		}
	}
	return nil, nil
}

func receiverMatches(fd *ast.FuncDecl, receiver string) bool {
	return receiver != "" && receiverTypeName(fd) == receiver
}

func importAliases(file *ast.File) (middlewareAliases, aclAliases []string) {
	if file == nil {
		return defaultedAliases(nil, "middleware"), defaultedAliases(nil, "acl")
	}
	for _, imp := range file.Imports {
		path := strings.Trim(imp.Path.Value, `"`)
		alias := ""
		if imp.Name != nil && imp.Name.Name != "" && imp.Name.Name != "_" && imp.Name.Name != "." {
			alias = imp.Name.Name
		} else {
			alias = filepath.Base(path)
		}

		if path == middlewareImportPath {
			middlewareAliases = append(middlewareAliases, alias)
		}
		if path == aclImportPath {
			aclAliases = append(aclAliases, alias)
		}
	}

	return defaultedAliases(middlewareAliases, "middleware"), defaultedAliases(aclAliases, "acl")
}

func defaultedAliases(aliases []string, fallback string) []string {
	if len(aliases) == 0 {
		return []string{fallback}
	}
	return aliases
}

func aliasMatches(name string, aliases []string) bool {
	for _, alias := range aliases {
		if name == alias {
			return true
		}
	}
	return false
}

func uniqueStrings(values []string) []string {
	seen := make(map[string]struct{}, len(values))
	var result []string
	for _, value := range values {
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		result = append(result, value)
	}
	return result
}

// extractACLPermissions inspects middleware function names for ACL hints.
func extractACLPermissions(middlewares []func(http.Handler) http.Handler) []string {
	var permissions []string

	for _, mw := range middlewares {
		funcName := runtime.FuncForPC(reflect.ValueOf(mw).Pointer()).Name()

		if permission := extractPermissionFromMiddleware(mw, funcName); permission != "" {
			permissions = append(permissions, permission)
		}
	}

	return permissions
}

// extractPermissionFromMiddleware provides human-readable descriptions from middleware names.
func extractPermissionFromMiddleware(mw func(http.Handler) http.Handler, funcName string) string {
	switch {
	case strings.Contains(funcName, "Can"):
		return "requires specific ACL permission"
	case strings.Contains(funcName, "Any"):
		return "requires any of multiple ACL permissions"
	case strings.Contains(funcName, "Must"):
		return "requires all specified ACL permissions"
	case strings.Contains(funcName, "IsSystemAdmin"):
		return "requires SystemAdmin role"
	case strings.Contains(funcName, "IsTenantAdmin"):
		return "requires TenantAdmin role"
	case strings.Contains(funcName, "IsTenant"):
		return "requires valid tenant context"
	case strings.Contains(funcName, "Authenticated"):
		return "requires valid authentication"
	default:
		return ""
	}
}

// inferPermissionFromRoute heuristically maps routes to ACL permissions.
func inferPermissionFromRoute(route, method string, middlewares []func(http.Handler) http.Handler) string {
	resource := extractResourceFromRoute(route)

	hasACLMiddleware := false
	aclType := ""

	for _, mw := range middlewares {
		funcName := runtime.FuncForPC(reflect.ValueOf(mw).Pointer()).Name()
		switch {
		case strings.Contains(funcName, "Can"):
			hasACLMiddleware = true
			aclType = "Can"
		case strings.Contains(funcName, "Any"):
			hasACLMiddleware = true
			aclType = "Any"
		case strings.Contains(funcName, "Must"):
			hasACLMiddleware = true
			aclType = "Must"
		}
		if hasACLMiddleware {
			break
		}
	}

	if !hasACLMiddleware {
		return ""
	}

	return inferPermissionFromContext(resource, method, aclType)
}

// inferPermissionFromContext builds a readable permission string.
func inferPermissionFromContext(resource, method, aclType string) string {
	resourceTitle := capitalize(resource)

	switch method {
	case http.MethodGet:
		return resourceTitle + "Read permission required"
	case http.MethodPost, http.MethodPut, http.MethodPatch:
		return resourceTitle + "Write permission required"
	case http.MethodDelete:
		return resourceTitle + "Delete permission required"
	default:
		return resourceTitle + " permission required"
	}
}
