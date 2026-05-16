package main

import (
	"embed"
	"errors"
	"fmt"
	"io/fs"
	"net"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"syscall"
)

const devServerAddr = "127.0.0.1:3971"
const createNoWindow = 0x08000000

//go:embed webroot/*
var webroot embed.FS

func main() {
	if err := run(); err != nil {
		_, _ = fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func run() error {
	core, err := corePath()
	if err != nil {
		return err
	}

	if len(os.Args) > 1 {
		return runCore(core, os.Args[1:], false)
	}

	listener, err := net.Listen("tcp", devServerAddr)
	if err != nil {
		return fmt.Errorf("start CC Switch local UI server on %s: %w", devServerAddr, err)
	}
	defer listener.Close()

	static, err := fs.Sub(webroot, "webroot")
	if err != nil {
		return fmt.Errorf("open embedded UI files: %w", err)
	}
	server := &http.Server{Handler: spaHandler(http.FS(static))}
	go func() {
		_ = server.Serve(listener)
	}()
	defer server.Close()

	return runCore(core, nil, true)
}

func corePath() (string, error) {
	exe, err := os.Executable()
	if err != nil {
		return "", fmt.Errorf("resolve wrapper path: %w", err)
	}
	core := filepath.Join(filepath.Dir(exe), "cc-switch-core.exe")
	if _, err := os.Stat(core); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return "", fmt.Errorf("cc-switch-core.exe was not found next to cc-switch.exe")
		}
		return "", err
	}
	return core, nil
}

func runCore(core string, args []string, hideWindow bool) error {
	cmd := exec.Command(core, args...)
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if hideWindow {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: createNoWindow,
			HideWindow:    true,
		}
	}
	if err := cmd.Run(); err != nil {
		return err
	}
	return nil
}

func spaHandler(root http.FileSystem) http.Handler {
	files := http.FileServer(root)
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := r.URL.Path
		if path == "" || path == "/" {
			files.ServeHTTP(w, r)
			return
		}
		if file, err := root.Open(path[1:]); err == nil {
			_ = file.Close()
			files.ServeHTTP(w, r)
			return
		}
		r.URL.Path = "/"
		files.ServeHTTP(w, r)
	})
}
