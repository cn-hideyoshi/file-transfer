package fileproxy

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	ListenAddr string
	RootDir    string
}

func LoadConfig() (Config, error) {
	defaultAddr := envOrDefault("LISTEN_ADDR", ":8080")
	defaultRoot := envOrDefault("FILE_ROOT", ".")

	addr := flag.String("addr", defaultAddr, "HTTP listen address")
	root := flag.String("root", defaultRoot, "local directory exposed by the server")
	flag.Parse()

	absRoot, err := filepath.Abs(*root)
	if err != nil {
		return Config{}, fmt.Errorf("resolve root dir: %w", err)
	}

	info, err := os.Stat(absRoot)
	if err != nil {
		return Config{}, fmt.Errorf("stat root dir: %w", err)
	}
	if !info.IsDir() {
		return Config{}, fmt.Errorf("root path %q is not a directory", absRoot)
	}

	return Config{
		ListenAddr: *addr,
		RootDir:    absRoot,
	}, nil
}

func envOrDefault(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}
