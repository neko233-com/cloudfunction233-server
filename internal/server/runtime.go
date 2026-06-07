package server

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/http"
	"os/exec"
	"path/filepath"
)

type Runtime interface {
	Invoke(ctx context.Context, fn Function, baseDir string, req *http.Request, remainingPath string) (InvocationResponse, error)
}

type InvocationResponse struct {
	Status  int               `json:"status"`
	Headers map[string]string `json:"headers"`
	Body    string            `json:"body"`
	Base64  bool              `json:"base64"`
	Meta    map[string]string `json:"meta,omitempty"`
	Raw     json.RawMessage   `json:"raw,omitempty"`
}

type RuntimeRegistry struct {
	runtimes map[string]Runtime
}

func NewRuntimeRegistry(nodeBin string) RuntimeRegistry {
	node := NodeRuntime{NodeBin: nodeBin}
	return RuntimeRegistry{runtimes: map[string]Runtime{
		"node":   node,
		"nodejs": node,
		"npm":    node,
		"static": StaticRuntime{},
	}}
}

func (r RuntimeRegistry) Get(name string) (Runtime, bool) {
	rt, ok := r.runtimes[name]
	return rt, ok
}

type NodeRuntime struct {
	NodeBin string
}

type StaticRuntime struct{}

func (StaticRuntime) Invoke(_ context.Context, fn Function, _ string, _ *http.Request, _ string) (InvocationResponse, error) {
	status := 200
	contentType := "text/plain; charset=utf-8"
	body := "ok"
	if fn.Env != nil {
		if value := fn.Env["status"]; value != "" {
			_, _ = fmt.Sscanf(value, "%d", &status)
		}
		if value := fn.Env["contentType"]; value != "" {
			contentType = value
		}
		if value := fn.Env["body"]; value != "" {
			body = value
		}
	}
	return InvocationResponse{
		Status:  status,
		Headers: map[string]string{"content-type": contentType},
		Body:    body,
	}, nil
}

type nodeRequest struct {
	Method        string              `json:"method"`
	URL           string              `json:"url"`
	Path          string              `json:"path"`
	RemainingPath string              `json:"remainingPath"`
	Headers       map[string][]string `json:"headers"`
	Query         string              `json:"query"`
	Body          string              `json:"body"`
	Base64        bool                `json:"base64"`
	Env           map[string]string   `json:"env"`
}

func (r NodeRuntime) Invoke(ctx context.Context, fn Function, baseDir string, req *http.Request, remainingPath string) (InvocationResponse, error) {
	body, err := readRequestBody(req)
	if err != nil {
		return InvocationResponse{}, err
	}
	payload := nodeRequest{
		Method:        req.Method,
		URL:           absoluteRequestURL(req),
		Path:          req.URL.Path,
		RemainingPath: remainingPath,
		Headers:       req.Header,
		Query:         req.URL.RawQuery,
		Body:          base64.StdEncoding.EncodeToString(body),
		Base64:        true,
		Env:           fn.Env,
	}
	input, err := json.Marshal(payload)
	if err != nil {
		return InvocationResponse{}, err
	}

	runner := filepath.Join(baseDir, ".runner.mjs")
	cmd := exec.CommandContext(ctx, r.NodeBin, runner, fn.Entrypoint, fn.Handler)
	cmd.Dir = baseDir
	cmd.Stdin = bytes.NewReader(input)
	var stdout bytes.Buffer
	var stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	if err := cmd.Run(); err != nil {
		return InvocationResponse{}, fmt.Errorf("node function failed: %w: %s", err, stderr.String())
	}
	var out InvocationResponse
	if err := json.Unmarshal(stdout.Bytes(), &out); err != nil {
		return InvocationResponse{}, fmt.Errorf("invalid node response: %w: %s", err, stdout.String())
	}
	if out.Status == 0 {
		out.Status = http.StatusOK
	}
	return out, nil
}

func readRequestBody(req *http.Request) ([]byte, error) {
	if req.Body == nil {
		return nil, nil
	}
	defer req.Body.Close()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(req.Body); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func absoluteRequestURL(req *http.Request) string {
	scheme := "http"
	if req.TLS != nil {
		scheme = "https"
	}
	return scheme + "://" + req.Host + req.URL.RequestURI()
}
