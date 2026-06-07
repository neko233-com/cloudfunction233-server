package server

import (
	"embed"
	"encoding/json"
	"io/fs"
	"net/http"
	"strings"
	"time"
)

//go:embed static/*
var adminStatic embed.FS

type loginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

func (s *Server) adminUI(w http.ResponseWriter, r *http.Request) {
	http.Redirect(w, r, "/admin/", http.StatusFound)
}

func (s *Server) adminAssets(w http.ResponseWriter, r *http.Request) {
	sub, err := fs.Sub(adminStatic, "static")
	if err != nil {
		writeError(w, http.StatusInternalServerError, err)
		return
	}
	http.StripPrefix("/admin/", http.FileServer(http.FS(sub))).ServeHTTP(w, r)
}

func (s *Server) login(w http.ResponseWriter, r *http.Request) {
	var req loginRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, err)
		return
	}
	if !s.store.Authenticate(req.Username, req.Password) {
		writeError(w, http.StatusUnauthorized, errUnauthorized())
		return
	}
	token := s.sessions.Create(req.Username)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		SameSite: http.SameSiteLaxMode,
		Expires:  time.Now().Add(24 * time.Hour),
	})
	writeJSONResponse(w, http.StatusOK, map[string]string{"username": req.Username})
}

func (s *Server) logout(w http.ResponseWriter, r *http.Request) {
	s.sessions.Delete(r)
	http.SetCookie(w, &http.Cookie{
		Name:     sessionCookieName,
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		MaxAge:   -1,
	})
	writeJSONResponse(w, http.StatusOK, map[string]string{"status": "ok"})
}

func errUnauthorized() error {
	return stringError("unauthorized")
}

type stringError string

func (e stringError) Error() string {
	return strings.TrimSpace(string(e))
}
