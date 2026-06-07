package server

import (
	"context"
	"errors"
	"log"
	"net"
	"net/http"
	"sync"
)

type App struct {
	cfg        Config
	handler    *Server
	httpServer *http.Server
}

func NewApp(cfg Config) (*App, error) {
	handler, err := newHandler(cfg)
	if err != nil {
		return nil, err
	}
	return &App{
		cfg:     cfg,
		handler: handler,
		httpServer: &http.Server{
			Addr:    ":" + cfg.Port,
			Handler: handler,
		},
	}, nil
}

func (a *App) ListenAndServe() error {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var wg sync.WaitGroup
	errCh := make(chan error, 3)

	wg.Add(1)
	go func() {
		defer wg.Done()
		log.Printf("http functions listening on :%s", a.cfg.Port)
		if err := a.httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
			cancel()
		}
	}()

	if a.cfg.EnableTCP {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.listenTCP(ctx); err != nil {
				errCh <- err
				cancel()
			}
		}()
	} else {
		log.Printf("tcp functions are disabled; reserved port :%s", a.cfg.TCPPort)
	}

	if a.cfg.EnableUDP {
		wg.Add(1)
		go func() {
			defer wg.Done()
			if err := a.listenUDP(ctx); err != nil {
				errCh <- err
				cancel()
			}
		}()
	} else {
		log.Printf("udp functions are disabled; reserved port :%s", a.cfg.UDPPort)
	}

	select {
	case err := <-errCh:
		_ = a.httpServer.Shutdown(context.Background())
		cancel()
		wg.Wait()
		return err
	case <-ctx.Done():
		wg.Wait()
		return nil
	}
}

func (a *App) listenTCP(ctx context.Context) error {
	ln, err := net.Listen("tcp", ":"+a.cfg.TCPPort)
	if err != nil {
		return err
	}
	defer ln.Close()
	log.Printf("tcp functions listening on :%s", a.cfg.TCPPort)

	go func() {
		<-ctx.Done()
		_ = ln.Close()
	}()

	for {
		conn, err := ln.Accept()
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		go a.handler.handleTCPConn(ctx, conn)
	}
}

func (a *App) listenUDP(ctx context.Context) error {
	addr, err := net.ResolveUDPAddr("udp", ":"+a.cfg.UDPPort)
	if err != nil {
		return err
	}
	conn, err := net.ListenUDP("udp", addr)
	if err != nil {
		return err
	}
	defer conn.Close()
	log.Printf("udp functions listening on :%s", a.cfg.UDPPort)

	go func() {
		<-ctx.Done()
		_ = conn.Close()
	}()

	buf := make([]byte, 64*1024)
	for {
		n, remote, err := conn.ReadFromUDP(buf)
		if err != nil {
			if ctx.Err() != nil {
				return nil
			}
			return err
		}
		packet := append([]byte(nil), buf[:n]...)
		go a.handler.handleUDPPacket(ctx, conn, remote, packet)
	}
}
