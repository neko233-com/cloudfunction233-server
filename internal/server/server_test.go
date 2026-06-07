package server

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
)

func TestAdminLoginAndFunctionCRUD(t *testing.T) {
	srv := newTestHTTPServer(t)
	defer srv.Close()

	adminResp, err := http.Get(srv.URL + "/admin/")
	if err != nil {
		t.Fatal(err)
	}
	defer adminResp.Body.Close()
	if adminResp.StatusCode != http.StatusOK {
		t.Fatalf("admin status = %d, want 200", adminResp.StatusCode)
	}

	loginBody := `{"username":"root","password":"root"}`
	loginResp, err := http.Post(srv.URL+"/api/v1/login", "application/json", strings.NewReader(loginBody))
	if err != nil {
		t.Fatal(err)
	}
	defer loginResp.Body.Close()
	if loginResp.StatusCode != http.StatusOK {
		t.Fatalf("login status = %d, want 200", loginResp.StatusCode)
	}
	if len(loginResp.Cookies()) == 0 {
		t.Fatal("login did not set a session cookie")
	}

	deploy := DeployRequest{
		Project: "demo",
		Name:    "hello",
		Type:    "http",
		Runtime: "static",
		Env: map[string]string{
			"body": "created",
		},
	}
	doJSON(t, srv.URL+"/api/v1/functions", http.MethodPost, deploy, http.StatusCreated)

	resp, err := http.Get(srv.URL + "/root/demo/hello")
	if err != nil {
		t.Fatal(err)
	}
	body := readBody(t, resp)
	if body != "created" {
		t.Fatalf("function body = %q, want created", body)
	}

	deploy.Env["body"] = "updated"
	doJSON(t, srv.URL+"/api/v1/functions/hello", http.MethodPut, deploy, http.StatusCreated)
	resp, err = http.Get(srv.URL + "/root/demo/hello")
	if err != nil {
		t.Fatal(err)
	}
	body = readBody(t, resp)
	if body != "updated" {
		t.Fatalf("updated body = %q, want updated", body)
	}

	req, err := http.NewRequest(http.MethodDelete, srv.URL+"/api/v1/functions/hello", nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", basicAuth())
	deleteResp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer deleteResp.Body.Close()
	if deleteResp.StatusCode != http.StatusNoContent {
		t.Fatalf("delete status = %d, want 204", deleteResp.StatusCode)
	}
}

func TestProjectsRuntimesAndTCPReservation(t *testing.T) {
	srv := newTestHTTPServer(t)
	defer srv.Close()

	doJSON(t, srv.URL+"/api/v1/functions", http.MethodPost, DeployRequest{
		Project: "ops",
		Name:    "ping",
		Type:    "http",
		Runtime: "static",
		Env:     map[string]string{"body": "pong"},
	}, http.StatusCreated)

	var projects []string
	getJSON(t, srv.URL+"/api/v1/projects", &projects)
	if len(projects) != 1 || projects[0] != "ops" {
		t.Fatalf("projects = %#v, want [ops]", projects)
	}

	var runtimes []RuntimeInfo
	getJSON(t, srv.URL+"/api/v1/runtimes", &runtimes)
	if !runtimeAvailable(runtimes, "static") {
		t.Fatal("static runtime should always be available")
	}
	if runtimeAvailable(runtimes, "node") {
		t.Fatal("node runtime should not be available without a runtime pack")
	}

	var tcp TCPProtocolConfig
	getJSON(t, srv.URL+"/api/v1/tcp-protocol", &tcp)
	if tcp.Enabled || !tcp.Reserved || tcp.LengthFieldType != "uint32" {
		t.Fatalf("unexpected tcp config: %#v", tcp)
	}
}

func TestRejectsReservedProtocolsAndMissingRuntimePacks(t *testing.T) {
	srv := newTestHTTPServer(t)
	defer srv.Close()

	doJSON(t, srv.URL+"/api/v1/functions", http.MethodPost, DeployRequest{
		Project: "demo",
		Name:    "tcp-fn",
		Type:    "tcp",
		Runtime: "static",
	}, http.StatusBadRequest)

	doJSON(t, srv.URL+"/api/v1/functions", http.MethodPost, DeployRequest{
		Project: "demo",
		Name:    "node-fn",
		Type:    "http",
		Runtime: "node",
	}, http.StatusBadRequest)
}

func newTestHTTPServer(t *testing.T) *httptest.Server {
	t.Helper()
	cfg := Config{
		Port:              "0",
		TCPPort:           "0",
		UDPPort:           "0",
		DataDir:           filepath.Join(t.TempDir(), "data"),
		RuntimeDir:        filepath.Join(t.TempDir(), "runtimes"),
		NodeBin:           "node",
		NPMBin:            "npm",
		DefaultUsername:   "root",
		DefaultPassword:   "root",
		InvocationTimeout: 0,
	}
	handler, err := newHandler(cfg)
	if err != nil {
		t.Fatal(err)
	}
	return httptest.NewServer(handler)
}

func doJSON(t *testing.T, url, method string, payload any, wantStatus int) {
	t.Helper()
	data, err := json.Marshal(payload)
	if err != nil {
		t.Fatal(err)
	}
	req, err := http.NewRequest(method, url, bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", basicAuth())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != wantStatus {
		t.Fatalf("%s %s status = %d, want %d; body=%s", method, url, resp.StatusCode, wantStatus, readBody(t, resp))
	}
}

func getJSON(t *testing.T, url string, target any) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	if err != nil {
		t.Fatal(err)
	}
	req.Header.Set("Authorization", basicAuth())
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s status = %d, want 200", url, resp.StatusCode)
	}
	if err := json.NewDecoder(resp.Body).Decode(target); err != nil {
		t.Fatal(err)
	}
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	var buf bytes.Buffer
	if _, err := buf.ReadFrom(resp.Body); err != nil {
		t.Fatal(err)
	}
	return buf.String()
}

func basicAuth() string {
	return "Basic " + base64.StdEncoding.EncodeToString([]byte("root:root"))
}

func runtimeAvailable(runtimes []RuntimeInfo, name string) bool {
	for _, runtime := range runtimes {
		if runtime.Name == name {
			return runtime.Available
		}
	}
	return false
}
