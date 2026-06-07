package server

import (
	"crypto/rand"
	"encoding/hex"
	"net/http"
	"sync"
	"time"
)

const sessionCookieName = "cf233_session"

type SessionStore struct {
	mu       sync.RWMutex
	sessions map[string]session
}

type session struct {
	Username  string
	ExpiresAt time.Time
}

func NewSessionStore() *SessionStore {
	return &SessionStore{sessions: map[string]session{}}
}

func (s *SessionStore) Create(username string) string {
	tokenBytes := make([]byte, 32)
	_, _ = rand.Read(tokenBytes)
	token := hex.EncodeToString(tokenBytes)
	s.mu.Lock()
	defer s.mu.Unlock()
	s.sessions[token] = session{Username: username, ExpiresAt: time.Now().Add(24 * time.Hour)}
	return token
}

func (s *SessionStore) Username(r *http.Request) (string, bool) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return "", false
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	session, ok := s.sessions[cookie.Value]
	if !ok || time.Now().After(session.ExpiresAt) {
		return "", false
	}
	return session.Username, true
}

func (s *SessionStore) Delete(r *http.Request) {
	cookie, err := r.Cookie(sessionCookieName)
	if err != nil {
		return
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.sessions, cookie.Value)
}
