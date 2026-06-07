package server

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

type Store struct {
	dir       string
	mu        sync.RWMutex
	tenants   map[string]Tenant
	functions map[string][]Function
}

func NewStore(dir, defaultUsername, defaultPassword string) (*Store, error) {
	s := &Store{
		dir:       dir,
		tenants:   map[string]Tenant{},
		functions: map[string][]Function{},
	}
	if err := os.MkdirAll(s.functionsDir(), 0755); err != nil {
		return nil, err
	}
	if err := s.loadTenants(); err != nil {
		return nil, err
	}
	if _, ok := s.tenants[defaultUsername]; !ok {
		s.tenants[defaultUsername] = Tenant{
			Username:     defaultUsername,
			PasswordHash: hashPassword(defaultPassword),
			CreatedAt:    time.Now().UTC(),
		}
		if err := s.saveTenantsLocked(); err != nil {
			return nil, err
		}
	}
	if err := s.loadFunctions(); err != nil {
		return nil, err
	}
	return s, nil
}

func (s *Store) Authenticate(username, password string) bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	tenant, ok := s.tenants[username]
	return ok && tenant.PasswordHash == hashPassword(password)
}

func (s *Store) CreateTenant(username, password string) (Tenant, error) {
	username = cleanSegment(username)
	if username == "" || password == "" {
		return Tenant{}, errors.New("username and password are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, exists := s.tenants[username]; exists {
		return Tenant{}, fmt.Errorf("tenant %q already exists", username)
	}
	tenant := Tenant{
		Username:     username,
		PasswordHash: hashPassword(password),
		CreatedAt:    time.Now().UTC(),
	}
	s.tenants[username] = tenant
	return tenant, s.saveTenantsLocked()
}

func (s *Store) ListTenants() []Tenant {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]Tenant, 0, len(s.tenants))
	for _, tenant := range s.tenants {
		out = append(out, tenant)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Username < out[j].Username })
	return out
}

func (s *Store) UpsertFunction(fn Function) (Function, error) {
	fn.Tenant = cleanSegment(fn.Tenant)
	fn.Project = cleanSegment(valueOrDefault(fn.Project, "default"))
	fn.Name = cleanSegment(fn.Name)
	if strings.TrimSpace(fn.Route) == "" && fn.Project != "" && fn.Name != "" {
		fn.Route = "/" + fn.Project + "/" + fn.Name
	}
	fn.Route = normalizeRoute(fn.Route)
	fn.Type = normalizeType(fn.Type)
	fn.Runtime = strings.ToLower(strings.TrimSpace(fn.Runtime))
	if fn.Entrypoint == "" {
		fn.Entrypoint = "index.js"
	}
	if fn.Handler == "" {
		fn.Handler = "fetch"
	}
	if fn.Tenant == "" || fn.Name == "" || fn.Route == "" || fn.Runtime == "" {
		return Function{}, errors.New("tenant, name, route, and runtime are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.tenants[fn.Tenant]; !ok {
		return Function{}, fmt.Errorf("tenant %q does not exist", fn.Tenant)
	}
	now := time.Now().UTC()
	fn.UpdatedAt = now
	list := s.functions[fn.Tenant]
	for i, existing := range list {
		if existing.Name == fn.Name {
			if fn.CreatedAt.IsZero() {
				fn.CreatedAt = existing.CreatedAt
			}
			list[i] = fn
			s.functions[fn.Tenant] = list
			return fn, s.saveFunctionLocked(fn)
		}
	}
	if fn.CreatedAt.IsZero() {
		fn.CreatedAt = now
	}
	s.functions[fn.Tenant] = append(list, fn)
	return fn, s.saveFunctionLocked(fn)
}

func (s *Store) ListFunctions(tenant string) []Function {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := append([]Function(nil), s.functions[tenant]...)
	sort.Slice(list, func(i, j int) bool { return list[i].Route < list[j].Route })
	return list
}

func (s *Store) GetFunctionByName(tenant, name string) (Function, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	name = cleanSegment(name)
	for _, fn := range s.functions[tenant] {
		if fn.Name == name {
			return fn, true
		}
	}
	return Function{}, false
}

func (s *Store) ListProjects(tenant string) []string {
	s.mu.RLock()
	defer s.mu.RUnlock()
	seen := map[string]bool{}
	for _, fn := range s.functions[tenant] {
		project := valueOrDefault(fn.Project, "default")
		seen[project] = true
	}
	out := make([]string, 0, len(seen))
	for project := range seen {
		out = append(out, project)
	}
	sort.Strings(out)
	return out
}

func (s *Store) DeleteFunction(tenant, name string) error {
	tenant = cleanSegment(tenant)
	name = cleanSegment(name)

	s.mu.Lock()
	defer s.mu.Unlock()
	list := s.functions[tenant]
	for i, fn := range list {
		if fn.Name == name {
			s.functions[tenant] = append(list[:i], list[i+1:]...)
			return os.RemoveAll(s.functionDir(fn))
		}
	}
	return fs.ErrNotExist
}

func (s *Store) MatchFunction(tenant, requestPath string) (Function, string, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	requestPath = normalizeRoute(requestPath)
	list := append([]Function(nil), s.functions[tenant]...)
	sort.Slice(list, func(i, j int) bool { return len(list[i].Route) > len(list[j].Route) })
	for _, fn := range list {
		if normalizeType(fn.Type) != "http" {
			continue
		}
		route := normalizeRoute(fn.Route)
		if requestPath == route || strings.HasPrefix(requestPath, route+"/") {
			rest := strings.TrimPrefix(requestPath, route)
			rest = "/" + strings.TrimPrefix(rest, "/")
			return fn, rest, true
		}
	}
	return Function{}, "", false
}

func (s *Store) GetFunction(tenant, name, fnType string) (Function, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	name = cleanSegment(name)
	fnType = normalizeType(fnType)
	for _, fn := range s.functions[tenant] {
		if fn.Name == name && normalizeType(fn.Type) == fnType {
			return fn, true
		}
	}
	return Function{}, false
}

func (s *Store) FunctionDir(fn Function) string {
	return s.functionDir(fn)
}

func (s *Store) SaveFunctionFiles(fn Function, files map[string]string) error {
	base := s.functionDir(fn)
	if err := os.MkdirAll(base, 0755); err != nil {
		return err
	}
	for rawPath, content := range files {
		cleaned, err := safeRelativePath(rawPath)
		if err != nil {
			return err
		}
		fullPath := filepath.Join(base, cleaned)
		if err := os.MkdirAll(filepath.Dir(fullPath), 0755); err != nil {
			return err
		}
		if err := os.WriteFile(fullPath, []byte(content), 0644); err != nil {
			return err
		}
	}
	return nil
}

func (s *Store) tenantsFile() string {
	return filepath.Join(s.dir, "tenants.json")
}

func (s *Store) functionsDir() string {
	return filepath.Join(s.dir, "functions")
}

func (s *Store) functionDir(fn Function) string {
	return filepath.Join(s.functionsDir(), cleanSegment(fn.Tenant), cleanSegment(valueOrDefault(fn.Project, "default")), cleanSegment(fn.Name))
}

func (s *Store) functionMetaFile(fn Function) string {
	return filepath.Join(s.functionDir(fn), "function.json")
}

func (s *Store) saveTenantsLocked() error {
	if err := os.MkdirAll(s.dir, 0755); err != nil {
		return err
	}
	list := make([]Tenant, 0, len(s.tenants))
	for _, tenant := range s.tenants {
		list = append(list, tenant)
	}
	sort.Slice(list, func(i, j int) bool { return list[i].Username < list[j].Username })
	return writeJSON(s.tenantsFile(), list)
}

func (s *Store) saveFunctionLocked(fn Function) error {
	if err := os.MkdirAll(s.functionDir(fn), 0755); err != nil {
		return err
	}
	return writeJSON(s.functionMetaFile(fn), fn)
}

func (s *Store) loadTenants() error {
	data, err := os.ReadFile(s.tenantsFile())
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return err
	}
	var list []Tenant
	if err := json.Unmarshal(data, &list); err != nil {
		return err
	}
	for _, tenant := range list {
		s.tenants[tenant.Username] = tenant
	}
	return nil
}

func (s *Store) loadFunctions() error {
	return filepath.WalkDir(s.functionsDir(), func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() || d.Name() != "function.json" {
			return err
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		var fn Function
		if err := json.Unmarshal(data, &fn); err != nil {
			return err
		}
		s.functions[fn.Tenant] = append(s.functions[fn.Tenant], fn)
		return nil
	})
}

func writeJSON(path string, value any) error {
	data, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, append(data, '\n'), 0644)
}

func hashPassword(password string) string {
	sum := sha256.Sum256([]byte(password))
	return hex.EncodeToString(sum[:])
}

func cleanSegment(value string) string {
	value = strings.TrimSpace(value)
	value = strings.Trim(value, "/\\")
	if value == "." || value == ".." || strings.ContainsAny(value, `/\`) {
		return ""
	}
	return value
}

func normalizeRoute(route string) string {
	route = strings.TrimSpace(route)
	route = "/" + strings.Trim(route, "/")
	if route == "/" {
		return route
	}
	return strings.TrimSuffix(route, "/")
}

func normalizeType(value string) string {
	value = strings.ToLower(strings.TrimSpace(value))
	if value == "" {
		return "http"
	}
	return value
}

func safeRelativePath(raw string) (string, error) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", errors.New("file path is required")
	}
	cleaned := filepath.Clean(strings.TrimLeft(raw, `/\`))
	if cleaned == "." || strings.HasPrefix(cleaned, "..") || filepath.IsAbs(cleaned) {
		return "", fmt.Errorf("unsafe file path %q", raw)
	}
	return cleaned, nil
}
