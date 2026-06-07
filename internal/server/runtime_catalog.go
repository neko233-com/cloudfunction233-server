package server

import (
	"os"
	"path/filepath"
	"runtime"
)

type RuntimeCatalog struct {
	dir string
}

func NewRuntimeCatalog(dir string) RuntimeCatalog {
	return RuntimeCatalog{dir: dir}
}

func (c RuntimeCatalog) List() []RuntimeInfo {
	items := []struct {
		name     string
		language string
		bin      string
	}{
		{"node", "JavaScript / TypeScript", filepath.Join("node", binName("node"))},
		{"npm", "Node.js package", filepath.Join("node", binName("node"))},
		{"python", "Python", filepath.Join("python", binName("python"))},
		{"go", "Go", filepath.Join("go", binName("go"))},
		{"java", "Java", filepath.Join("jre", "bin", binName("java"))},
		{"kotlin", "Kotlin", filepath.Join("kotlin", binName("kotlin"))},
	}
	out := []RuntimeInfo{{
		Name:      "static",
		Language:  "Built-in HTTP response",
		Available: true,
		Builtin:   true,
	}}
	for _, item := range items {
		path := filepath.Join(c.platformDir(), item.bin)
		_, err := os.Stat(path)
		info := RuntimeInfo{
			Name:      item.name,
			Language:  item.language,
			Available: err == nil,
			Builtin:   err == nil,
			Path:      path,
		}
		if err != nil {
			info.Path = ""
			info.Reason = "runtime pack not installed"
		}
		out = append(out, info)
	}
	return out
}

func (c RuntimeCatalog) IsAvailable(name string) bool {
	for _, info := range c.List() {
		if info.Name == name {
			return info.Available
		}
	}
	return false
}

func (c RuntimeCatalog) NodeBin() string {
	path := filepath.Join(c.platformDir(), "node", binName("node"))
	if _, err := os.Stat(path); err == nil {
		return path
	}
	return ""
}

func (c RuntimeCatalog) platformDir() string {
	return filepath.Join(c.dir, runtime.GOOS+"-"+runtime.GOARCH)
}

func binName(name string) string {
	if runtime.GOOS == "windows" {
		return name + ".exe"
	}
	return name
}
