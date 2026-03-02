package httpapi

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sort"
	"strings"
	"testing"

	"audistro-catalog/internal/apidocs"
)

type endpoint struct {
	Method string
	Path   string
}

func TestOpenAPISpecCoversRegisteredEndpoints(t *testing.T) {
	registered, err := registeredAPIEndpoints()
	if err != nil {
		t.Fatalf("read registered endpoints: %v", err)
	}
	specOps, err := parseOpenAPIOperations()
	if err != nil {
		t.Fatalf("parse openapi operations: %v", err)
	}

	missing := make([]string, 0)
	for _, ep := range registered {
		if ep.Path == "/healthz" || ep.Path == "/readyz" {
			continue
		}
		methods := specOps[normalizePath(ep.Path)]
		if methods == nil || !methods[ep.Method] {
			missing = append(missing, fmt.Sprintf("%s %s", ep.Method, ep.Path))
		}
	}
	sort.Strings(missing)
	if len(missing) > 0 {
		t.Fatalf("openapi spec is missing endpoint(s):\n%s", strings.Join(missing, "\n"))
	}
}

func registeredAPIEndpoints() ([]endpoint, error) {
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		return nil, fmt.Errorf("locate current file")
	}
	routesFile := filepath.Join(filepath.Dir(currentFile), "handlers", "routes.go")
	content, err := os.ReadFile(routesFile)
	if err != nil {
		return nil, err
	}
	src := string(content)

	found := make(map[endpoint]struct{})
	methodRoute := regexp.MustCompile(`mux\.Handle(?:Func)?\("([A-Z]+) ([^"]+)"`)
	for _, m := range methodRoute.FindAllStringSubmatch(src, -1) {
		if len(m) != 3 {
			continue
		}
		found[endpoint{Method: m[1], Path: normalizePath(m[2])}] = struct{}{}
	}

	out := make([]endpoint, 0, len(found))
	for ep := range found {
		out = append(out, ep)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Path == out[j].Path {
			return out[i].Method < out[j].Method
		}
		return out[i].Path < out[j].Path
	})
	return out, nil
}

func parseOpenAPIOperations() (map[string]map[string]bool, error) {
	spec, err := apidocs.LoadSpec()
	if err != nil {
		return nil, err
	}
	out := make(map[string]map[string]bool)
	for path, item := range spec.Paths.Map() {
		methods := make(map[string]bool)
		if item.Get != nil {
			methods["GET"] = true
		}
		if item.Post != nil {
			methods["POST"] = true
		}
		if item.Put != nil {
			methods["PUT"] = true
		}
		if item.Patch != nil {
			methods["PATCH"] = true
		}
		if item.Delete != nil {
			methods["DELETE"] = true
		}
		if item.Head != nil {
			methods["HEAD"] = true
		}
		if len(methods) > 0 {
			out[normalizePath(path)] = methods
		}
	}
	return out, nil
}

func normalizePath(path string) string {
	if path == "/" {
		return path
	}
	path = strings.TrimSuffix(path, "/")
	path = strings.ReplaceAll(path, "{targetId...}", "{targetId}")
	return path
}
