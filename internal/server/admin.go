package server

import (
	"errors"
	"strings"
)

func (s *Store) SetPassword(username, password string) error {
	username = cleanSegment(username)
	if username == "" || strings.TrimSpace(password) == "" {
		return errors.New("username and password are required")
	}

	s.mu.Lock()
	defer s.mu.Unlock()
	tenant, ok := s.tenants[username]
	if !ok {
		return errors.New("tenant not found")
	}
	tenant.PasswordHash = hashPassword(password)
	s.tenants[username] = tenant
	return s.saveTenantsLocked()
}
