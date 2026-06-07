package server

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io/fs"
	"net/http"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"
)

type Server struct {
	store    *Store
	mux      *http.ServeMux
	hot      *HotConfig
	sessions *SessionStore
	runtimes RuntimeCatalog
}

func newHandler(cfg Config) (*Server, error) {
	store, err := NewStore(cfg.DataDir, cfg.DefaultUsername, cfg.DefaultPassword)
	if err != nil {
		return nil, err
	}
	s := &Server{
		store:    store,
		mux:      http.NewServeMux(),
		hot:      NewHotConfig(configPath(), cfg),
		sessions: NewSessionStore(),
		runtimes: NewRuntimeCatalog(cfg.RuntimeDir),
	}
	stopHotReload := make(chan struct{})
	go s.hot.Watch(stopHotReload)
	s.routes()
	return s, nil
}

func New(cfg Config) (*http.Server, error) {
	s, err := newHandler(cfg)
	if err != nil {
		return nil, err
	}
	return &http.Server{
		Addr:    ":" + cfg.Port,
		Handler: s,
	}, nil
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	s.mux.ServeHTTP(w, r)
}

func (s *Server) routes() {
	s.mux.HandleFunc("GET /healthz", s.health)
	s.mux.HandleFunc("GET /admin", s.adminUI)
	s.mux.HandleFunc("GET /admin/", s.adminAssets)
	s.mux.HandleFunc("POST /api/v1/login", s.login)
	s.mux.HandleFunc("POST /api/v1/logout", s.logout)
	s.mux.HandleFunc("GET /api/v1/me", s.requireAuth(s.me))
	s.mux.HandleFunc("GET /api/v1/runtimes", s.requireAuth(s.listRuntimes))
	s.mux.HandleFunc("GET /api/v1/tcp-protocol", s.requireAuth(s.tcpProtocolConfig))
	s.mux.HandleFunc("GET /api/v1/tenants", s.requireAuth(s.listTenants))
	s.mux.HandleFunc("POST /api/v1/tenants", s.requireAuth(s.createTenant))
	s.mux.HandleFunc("GET /api/v1/projects", s.requireAuth(s.listProjects))
	s.mux.HandleFunc("GET /api/v1/functions", s.requireAuth(s.listFunctions))
	s.mux.HandleFunc("POST /api/v1/functions", s.requireAuth(s.deployFunction))
	s.mux.HandleFunc("GET /api/v1/functions/{name}", s.requireAuth(s.getFunction))
	s.mux.HandleFunc("PUT /api/v1/functions/{name}", s.requireAuth(s.updateFunction))
	s.mux.HandleFunc("DELETE /api/v1/functions/{name}", s.requireAuth(s.deleteFunction))
	s.mux.HandleFunc("/", s.invokeFunction)
}

func (s *Server) health(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func (s *Server) listTenants(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, s.store.ListTenants())
}

func (s *Server) createTenant(w http.ResponseWriter, r *http.Request) {
	var req CreateTenantRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	tenant, err := s.store.CreateTenant(req.Username, req.Password)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	writeJSONResponse(w, http.StatusCreated, tenant)
}

func (s *Server) me(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(w, http.StatusOK, map[string]string{"username": authenticatedUsername(r)})
}

func (s *Server) listRuntimes(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, s.runtimes.List())
}

func (s *Server) tcpProtocolConfig(w http.ResponseWriter, _ *http.Request) {
	writeJSONResponse(w, http.StatusOK, TCPProtocolConfig{
		Enabled:         false,
		FrameMode:       "length-prefix",
		LengthFieldType: "uint32",
		LengthFieldSize: 4,
		LengthEndian:    "big",
		MaxFrameBytes:   1048576,
		Reserved:        true,
	})
}

func (s *Server) listProjects(w http.ResponseWriter, r *http.Request) {
	writeJSONResponse(w, http.StatusOK, s.store.ListProjects(authenticatedUsername(r)))
}

func (s *Server) listFunctions(w http.ResponseWriter, r *http.Request) {
	username := authenticatedUsername(r)
	writeJSONResponse(w, http.StatusOK, s.store.ListFunctions(username))
}

func (s *Server) deployFunction(w http.ResponseWriter, r *http.Request) {
	username := authenticatedUsername(r)
	var req DeployRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	s.deployFunctionWithRequest(w, r.Context(), username, req)
}

func (s *Server) deployFunctionWithRequest(w http.ResponseWriter, ctx context.Context, username string, req DeployRequest) {
	fn := Function{
		Tenant:     username,
		Project:    req.Project,
		Name:       req.Name,
		Type:       req.Type,
		Route:      req.Route,
		Runtime:    valueOrDefault(req.Runtime, "npm"),
		Entrypoint: valueOrDefault(req.Entrypoint, "index.js"),
		Handler:    valueOrDefault(req.Handler, "fetch"),
		Env:        req.Env,
	}
	if normalizeType(fn.Type) != "http" {
		writeError(w, http.StatusBadRequest, errors.New("only http functions are enabled; tcp/udp protocol configuration is reserved"))
		return
	}
	if !s.runtimes.IsAvailable(strings.ToLower(fn.Runtime)) {
		writeError(w, http.StatusBadRequest, errors.New("runtime is not available without an installed runtime pack: "+fn.Runtime))
		return
	}
	stored, err := s.store.UpsertFunction(fn)
	if err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	fn = stored
	if len(req.Files) > 0 {
		if err := s.store.SaveFunctionFiles(fn, req.Files); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	if err := s.ensureNodeRunner(fn); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	if req.Install {
		if err := s.npmInstall(ctx, fn); err != nil {
			writeError(w, http.StatusBadRequest, err)
			return
		}
	}
	writeJSONResponse(w, http.StatusCreated, fn)
}

func (s *Server) getFunction(w http.ResponseWriter, r *http.Request) {
	fn, ok := s.store.GetFunctionByName(authenticatedUsername(r), r.PathValue("name"))
	if !ok {
		writeError(w, http.StatusNotFound, fs.ErrNotExist)
		return
	}
	writeJSONResponse(w, http.StatusOK, fn)
}

func (s *Server) updateFunction(w http.ResponseWriter, r *http.Request) {
	var req DeployRequest
	if err := decodeJSON(r, &req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	req.Name = r.PathValue("name")
	s.deployFunctionWithRequest(w, r.Context(), authenticatedUsername(r), req)
}

func (s *Server) deleteFunction(w http.ResponseWriter, r *http.Request) {
	if err := s.store.DeleteFunction(authenticatedUsername(r), r.PathValue("name")); err != nil {
		status := http.StatusInternalServerError
		if errors.Is(err, fs.ErrNotExist) {
			status = http.StatusNotFound
		}
		writeError(w, status, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) invokeFunction(w http.ResponseWriter, r *http.Request) {
	tenant, userPath, ok := splitTenantPath(r.URL.Path)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("request path must be /{username}/{path}"))
		return
	}
	fn, remainingPath, ok := s.store.MatchFunction(tenant, userPath)
	if !ok {
		writeError(w, http.StatusNotFound, errors.New("function route not found"))
		return
	}
	runtime, ok := s.runtimeFor(fn.Runtime)
	if !ok {
		writeError(w, http.StatusBadGateway, errors.New("unsupported runtime: "+fn.Runtime))
		return
	}
	if err := s.ensureNodeRunner(fn); err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), s.hot.Settings().InvocationTimeout)
	defer cancel()
	resp, err := runtime.Invoke(ctx, fn, s.store.FunctionDir(fn), r, remainingPath)
	if err != nil {
		writeError(w, http.StatusBadGateway, err)
		return
	}
	for key, value := range resp.Headers {
		w.Header().Set(key, value)
	}
	body := []byte(resp.Body)
	if resp.Base64 {
		decoded, err := base64.StdEncoding.DecodeString(resp.Body)
		if err != nil {
			writeError(w, http.StatusBadGateway, err)
			return
		}
		body = decoded
	}
	w.WriteHeader(resp.Status)
	_, _ = w.Write(body)
}

func (s *Server) requireAuth(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		username, password, ok := r.BasicAuth()
		if ok && s.store.Authenticate(username, password) {
			next(w, r.WithContext(context.WithValue(r.Context(), authUsernameKey{}, username)))
			return
		}
		if username, ok := s.sessions.Username(r); ok {
			next(w, r.WithContext(context.WithValue(r.Context(), authUsernameKey{}, username)))
			return
		}
		{
			w.Header().Set("WWW-Authenticate", `Basic realm="cloudfunction233"`)
			writeError(w, http.StatusUnauthorized, errors.New("unauthorized"))
			return
		}
	}
}

func (s *Server) ensureNodeRunner(fn Function) error {
	if strings.ToLower(fn.Runtime) != "npm" && strings.ToLower(fn.Runtime) != "node" && strings.ToLower(fn.Runtime) != "nodejs" {
		return nil
	}
	dir := s.store.FunctionDir(fn)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, ".runner.mjs"), []byte(nodeRunnerSource), 0644)
}

func (s *Server) npmInstall(ctx context.Context, fn Function) error {
	dir := s.store.FunctionDir(fn)
	if _, err := os.Stat(filepath.Join(dir, "package.json")); err != nil {
		return err
	}
	cmd := exec.CommandContext(ctx, s.hot.Settings().NPMBin, "install", "--omit=dev")
	cmd.Dir = dir
	output, err := cmd.CombinedOutput()
	if err != nil {
		return errors.New(string(output))
	}
	return nil
}

func (s *Server) runtimeFor(name string) (Runtime, bool) {
	nodeBin := s.runtimes.NodeBin()
	if nodeBin == "" {
		nodeBin = s.hot.Settings().NodeBin
	}
	return NewRuntimeRegistry(nodeBin).Get(strings.ToLower(name))
}

type authUsernameKey struct{}

func authenticatedUsername(r *http.Request) string {
	if username, ok := r.Context().Value(authUsernameKey{}).(string); ok {
		return username
	}
	return ""
}

func splitTenantPath(rawPath string) (string, string, bool) {
	cleaned := path.Clean("/" + strings.TrimPrefix(rawPath, "/"))
	parts := strings.Split(strings.TrimPrefix(cleaned, "/"), "/")
	if len(parts) < 2 || parts[0] == "" {
		return "", "", false
	}
	return parts[0], "/" + strings.Join(parts[1:], "/"), true
}

func decodeJSON(r *http.Request, value any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(value)
}

func writeJSONResponse(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSONResponse(w, status, ErrorResponse{Error: err.Error()})
}
