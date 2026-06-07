package server

import "time"

type Tenant struct {
	Username     string    `json:"username"`
	PasswordHash string    `json:"passwordHash"`
	CreatedAt    time.Time `json:"createdAt"`
}

type Function struct {
	Tenant     string            `json:"tenant"`
	Project    string            `json:"project"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Route      string            `json:"route"`
	Runtime    string            `json:"runtime"`
	Entrypoint string            `json:"entrypoint"`
	Handler    string            `json:"handler"`
	Env        map[string]string `json:"env,omitempty"`
	CreatedAt  time.Time         `json:"createdAt"`
	UpdatedAt  time.Time         `json:"updatedAt"`
}

type DeployRequest struct {
	Project    string            `json:"project"`
	Name       string            `json:"name"`
	Type       string            `json:"type"`
	Route      string            `json:"route"`
	Runtime    string            `json:"runtime"`
	Entrypoint string            `json:"entrypoint"`
	Handler    string            `json:"handler"`
	Env        map[string]string `json:"env"`
	Files      map[string]string `json:"files"`
	Install    bool              `json:"install"`
}

type RuntimeInfo struct {
	Name      string `json:"name"`
	Language  string `json:"language"`
	Available bool   `json:"available"`
	Builtin   bool   `json:"builtin"`
	Path      string `json:"path,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

type TCPProtocolConfig struct {
	Enabled         bool   `json:"enabled"`
	FrameMode       string `json:"frameMode"`
	LengthFieldType string `json:"lengthFieldType"`
	LengthFieldSize int    `json:"lengthFieldSize"`
	LengthEndian    string `json:"lengthEndian"`
	MaxFrameBytes   int    `json:"maxFrameBytes"`
	Reserved        bool   `json:"reserved"`
}

type CreateTenantRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}
