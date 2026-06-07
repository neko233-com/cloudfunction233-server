package cli

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"cloudfunction233-server/internal/server"
)

func Run(args []string) int {
	if len(args) == 0 {
		args = []string{"serve"}
	}
	exe, _ := os.Executable()
	cmd := args[0]

	switch cmd {
	case "serve":
		return serve()
	case "start":
		if err := Start(exe, args[1:]); err != nil {
			return fail(err)
		}
		fmt.Println("started")
	case "stop":
		if err := Stop(); err != nil {
			return fail(err)
		}
		fmt.Println("stopped")
	case "restart":
		if err := Restart(exe, args[1:]); err != nil {
			return fail(err)
		}
		fmt.Println("restarted")
	case "status":
		running, pid := Status()
		if running {
			fmt.Printf("running pid=%d\n", pid)
		} else {
			fmt.Println("stopped")
		}
	case "autostart":
		return autostart(args[1:], exe)
	case "update":
		return update(args[1:], exe)
	case "passwd":
		return passwd(args[1:])
	case "init-config":
		cfg := server.ConfigFromFileAndEnv()
		if err := server.EnsureConfigFile(configFile(), cfg); err != nil {
			return fail(err)
		}
		fmt.Println(configFile())
	case "help", "-h", "--help":
		usage()
	default:
		fmt.Fprintf(os.Stderr, "unknown command: %s\n", cmd)
		usage()
		return 2
	}
	return 0
}

func serve() int {
	cfg := server.ConfigFromFileAndEnv()
	if err := server.EnsureConfigFile(configPathForServe(), cfg); err != nil {
		return fail(err)
	}
	app, err := server.NewApp(cfg)
	if err != nil {
		return fail(err)
	}
	fmt.Printf("cloudfunction233-server listening http=:%s tcp=:%s udp=:%s data=%s\n", cfg.Port, cfg.TCPPort, cfg.UDPPort, cfg.DataDir)
	if err := app.ListenAndServe(); err != nil {
		return fail(err)
	}
	return 0
}

func autostart(args []string, exe string) int {
	if len(args) == 0 {
		fmt.Fprintln(os.Stderr, "usage: cloudfunction233-server autostart enable|disable")
		return 2
	}
	switch args[0] {
	case "enable":
		if err := EnableAutostart(exe); err != nil {
			return fail(err)
		}
		fmt.Println("autostart enabled")
	case "disable":
		if err := DisableAutostart(); err != nil {
			return fail(err)
		}
		fmt.Println("autostart disabled")
	default:
		fmt.Fprintln(os.Stderr, "usage: cloudfunction233-server autostart enable|disable")
		return 2
	}
	return 0
}

func update(args []string, exe string) int {
	fs := flag.NewFlagSet("update", flag.ContinueOnError)
	repo := fs.String("repo", "", "GitHub repo owner/name")
	url := fs.String("url", "", "direct download URL")
	noRestart := fs.Bool("no-restart", false, "do not restart after update")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if err := Update(*repo, *url, exe, !*noRestart); err != nil {
		return fail(err)
	}
	fmt.Println("updated")
	return 0
}

func passwd(args []string) int {
	fs := flag.NewFlagSet("passwd", flag.ContinueOnError)
	user := fs.String("user", "root", "tenant username")
	password := fs.String("password", "", "new password")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if *password == "" {
		fmt.Fprintln(os.Stderr, "--password is required")
		return 2
	}
	cfg := server.ConfigFromFileAndEnv()
	store, err := server.NewStore(cfg.DataDir, cfg.DefaultUsername, cfg.DefaultPassword)
	if err != nil {
		return fail(err)
	}
	if err := store.SetPassword(*user, *password); err != nil {
		return fail(err)
	}
	fmt.Printf("password updated for %s\n", *user)
	return 0
}

func usage() {
	fmt.Println(`cloudfunction233-server

Usage:
  cloudfunction233-server serve
  cloudfunction233-server start|stop|restart|status
  cloudfunction233-server autostart enable|disable
  cloudfunction233-server update --repo owner/name
  cloudfunction233-server update --url https://.../asset.zip
  cloudfunction233-server passwd --user root --password new-password
  cloudfunction233-server init-config`)
}

func fail(err error) int {
	fmt.Fprintln(os.Stderr, "error:", err)
	return 1
}

func configPathForServe() string {
	if path := os.Getenv("CF233_CONFIG"); path != "" {
		return path
	}
	if filepath.IsAbs(configFile()) {
		return configFile()
	}
	return ""
}
