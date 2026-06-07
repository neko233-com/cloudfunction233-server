package server

import (
	"bufio"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"strings"
)

type ProtocolRequest struct {
	Tenant  string            `json:"tenant"`
	Name    string            `json:"name"`
	Body    string            `json:"body"`
	Base64  bool              `json:"base64"`
	Headers map[string]string `json:"headers,omitempty"`
}

func (s *Server) handleTCPConn(parent context.Context, conn net.Conn) {
	defer conn.Close()
	scanner := bufio.NewScanner(conn)
	scanner.Buffer(make([]byte, 4096), 1024*1024)
	for scanner.Scan() {
		resp := s.invokeProtocol(parent, "tcp", scanner.Bytes(), conn.RemoteAddr().String())
		_ = json.NewEncoder(conn).Encode(resp)
	}
}

func (s *Server) handleUDPPacket(parent context.Context, conn *net.UDPConn, remote *net.UDPAddr, packet []byte) {
	resp := s.invokeProtocol(parent, "udp", packet, remote.String())
	data, err := json.Marshal(resp)
	if err != nil {
		return
	}
	_, _ = conn.WriteToUDP(data, remote)
}

func (s *Server) invokeProtocol(parent context.Context, fnType string, data []byte, remote string) InvocationResponse {
	var req ProtocolRequest
	cleanData := bytes.TrimPrefix(bytes.TrimSpace(data), []byte{0xef, 0xbb, 0xbf})
	if err := json.Unmarshal(cleanData, &req); err != nil {
		return protocolError(http.StatusBadRequest, err)
	}
	if req.Tenant == "" || req.Name == "" {
		return protocolError(http.StatusBadRequest, errors.New("tenant and name are required"))
	}
	fn, ok := s.store.GetFunction(req.Tenant, req.Name, fnType)
	if !ok {
		return protocolError(http.StatusNotFound, errors.New("function not found"))
	}
	runtime, ok := s.runtimeFor(fn.Runtime)
	if !ok {
		return protocolError(http.StatusBadGateway, errors.New("unsupported runtime: "+fn.Runtime))
	}
	if err := s.ensureNodeRunner(fn); err != nil {
		return protocolError(http.StatusInternalServerError, err)
	}

	body := []byte(req.Body)
	if req.Base64 {
		decoded, err := base64.StdEncoding.DecodeString(req.Body)
		if err != nil {
			return protocolError(http.StatusBadRequest, err)
		}
		body = decoded
	}

	ctx, cancel := context.WithTimeout(parent, s.hot.Settings().InvocationTimeout)
	defer cancel()

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, fnType+"://"+remote+"/"+req.Tenant+"/"+req.Name, bytes.NewReader(body))
	if err != nil {
		return protocolError(http.StatusInternalServerError, err)
	}
	for key, value := range req.Headers {
		httpReq.Header.Set(key, value)
	}
	httpReq.Header.Set("X-CF233-Protocol", strings.ToUpper(fnType))
	httpReq.Header.Set("X-CF233-Remote-Addr", remote)

	resp, err := runtime.Invoke(ctx, fn, s.store.FunctionDir(fn), httpReq, "/")
	if err != nil {
		return protocolError(http.StatusBadGateway, err)
	}
	return resp
}

func protocolError(status int, err error) InvocationResponse {
	return InvocationResponse{
		Status:  status,
		Headers: map[string]string{"content-type": "application/json; charset=utf-8"},
		Body:    `{"error":` + strconvQuote(err.Error()) + `}`,
	}
}

func strconvQuote(value string) string {
	data, _ := json.Marshal(value)
	return string(data)
}
