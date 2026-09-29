package router

import (
	"bufio"
	"net/http"
	"os"
	"regexp"
	"sort"
	"strings"
	"testing"

	"ascenda/internal/handler"
	"ascenda/internal/middleware"
	"github.com/go-chi/chi/v5"
	"github.com/sirupsen/logrus"
)

// TestOpenAPIMatchesRouter keeps docs/openapi.yaml honest against the router,
// which is authoritative:
//   - every operation in the spec must exist in the router (no stale docs);
//   - every route must be in the spec or in testdata/undocumented_routes.list,
//     so a new route cannot land undocumented;
//   - every line of that baseline must still be an undocumented route, so the
//     list only shrinks as routes get documented.
func TestOpenAPIMatchesRouter(t *testing.T) {
	routes := routerOperations(t)
	spec := specOperations(t, "../../docs/openapi.yaml")
	baseline := readOperationList(t, "testdata/undocumented_routes.list")

	for _, op := range sortedKeys(spec) {
		if !routes[op] {
			t.Errorf("docs/openapi.yaml documents %q, which the router does not serve", op)
		}
	}
	for _, op := range sortedKeys(routes) {
		if !spec[op] && !baseline[op] {
			t.Errorf("route %q is not documented: add it to docs/openapi.yaml", op)
		}
	}
	for _, op := range sortedKeys(baseline) {
		switch {
		case !routes[op]:
			t.Errorf("testdata/undocumented_routes.list lists %q, which the router no longer serves: delete the line", op)
		case spec[op]:
			t.Errorf("%q is now documented: delete its line from testdata/undocumented_routes.list", op)
		}
	}
}

// routerOperations builds the production router, with every optional route
// enabled, and returns its operations as normalised "METHOD path" keys.
func routerOperations(t *testing.T) map[string]bool {
	t.Helper()
	// Handlers are never invoked: Walk only reads the routing tree.
	r := newProductionRouter(&handler.HandlerBundle{}, middleware.TrustedProxies{})

	ops := map[string]bool{}
	err := chi.Walk(r, func(method, route string, _ http.Handler, _ ...func(http.Handler) http.Handler) error {
		ops[normaliseOperation(method, route)] = true
		return nil
	})
	if err != nil {
		t.Fatalf("walk router: %v", err)
	}
	return ops
}

// newProductionRouter builds the router exactly as cmd/server does, with the
// given handlers and trusted proxies, every optional route enabled, and
// middlewares whose dependencies are left empty.
func newProductionRouter(handlers *handler.HandlerBundle, trusted middleware.TrustedProxies) *chi.Mux {
	lg := logrus.New()
	lg.SetOutput(nopWriter{})
	le := logrus.NewEntry(lg)
	return NewRouter(handlers,
		middleware.NewAuthMiddleware("http://127.0.0.1:1/jwks", "http://issuer.test", "client", true, le),
		middleware.NewTenantMiddleware(nil, nil, le, nil, false),
		middleware.NewRBACMiddleware(le),
		middleware.NewPlanAccessMiddleware(nil, nil, nil, le),
		middleware.NewTierGateMiddleware(le),
		middleware.NewAIAccessMiddleware(nil, le),
		middleware.NewResourceScopeMiddleware(nil, nil, nil, le),
		middleware.NewLoggerMiddleware(lg),
		middleware.NewRecoverMiddleware(lg),
		middleware.NewRequestIDMiddleware(),
		middleware.NewSecurityHeadersMiddleware(),
		le, nil, 1<<20, true, trusted,
	)
}

var (
	specPathLine   = regexp.MustCompile(`^  (/\S*):\s*$`)
	specMethodLine = regexp.MustCompile(`^    (get|put|post|delete|patch|head|options):\s*$`)
)

// specOperations reads the operations under `paths:` in an OpenAPI YAML file.
// The file is indented with two spaces, so a line scan is enough and keeps
// a YAML parser out of the module's direct dependencies.
func specOperations(t *testing.T, path string) map[string]bool {
	t.Helper()
	f, err := os.Open(path)
	if err != nil {
		t.Fatalf("open spec: %v", err)
	}
	defer f.Close()

	ops := map[string]bool{}
	inPaths, current := false, ""
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := sc.Text()
		switch {
		case line == "paths:":
			inPaths = true
		case inPaths && line != "" && line[0] != ' ' && line[0] != '#':
			inPaths = false
		case !inPaths:
		case specPathLine.MatchString(line):
			current = specPathLine.FindStringSubmatch(line)[1]
		case specMethodLine.MatchString(line) && current != "":
			ops[normaliseOperation(specMethodLine.FindStringSubmatch(line)[1], current)] = true
		}
	}
	if err := sc.Err(); err != nil {
		t.Fatalf("read spec: %v", err)
	}
	if len(ops) == 0 {
		t.Fatalf("no operations found under paths: in %s", path)
	}
	return ops
}

func readOperationList(t *testing.T, path string) map[string]bool {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	ops := map[string]bool{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		method, route, ok := strings.Cut(line, " ")
		if !ok {
			t.Fatalf("%s: malformed line %q (want: METHOD path)", path, line)
		}
		ops[normaliseOperation(method, route)] = true
	}
	return ops
}

var pathParam = regexp.MustCompile(`\{[^}]+\}`)

// normaliseOperation makes router and spec paths comparable: parameter names
// are dropped and the trailing slash chi adds to sub-router roots is trimmed
// (chi serves /plans/{id} and /plans/{id}/ alike).
func normaliseOperation(method, route string) string {
	route = pathParam.ReplaceAllString(route, "{}")
	if len(route) > 1 {
		route = strings.TrimSuffix(route, "/")
	}
	return strings.ToUpper(method) + " " + route
}

func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

type nopWriter struct{}

func (nopWriter) Write(p []byte) (int, error) { return len(p), nil }
