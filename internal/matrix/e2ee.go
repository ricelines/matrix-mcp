package matrix

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	_ "github.com/mattn/go-sqlite3"
)

const pickleKeySize = 32

func pickleKeyPathForDB(dbPath string) string {
	return dbPath + ".pickle_key"
}

func loadOrCreatePickleKey(path string) ([]byte, error) {
	data, err := os.ReadFile(path)
	if err == nil {
		return parsePickleKey(data, path)
	}
	if !errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("read pickle key: %w", err)
	}

	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create pickle key dir: %w", err)
	}

	key := make([]byte, pickleKeySize)
	if _, err := rand.Read(key); err != nil {
		return nil, fmt.Errorf("generate pickle key: %w", err)
	}

	tmp, err := os.CreateTemp(filepath.Dir(path), filepath.Base(path)+".tmp-*")
	if err != nil {
		return nil, fmt.Errorf("create pickle key temp file: %w", err)
	}
	tmpPath := tmp.Name()
	defer func() {
		_ = os.Remove(tmpPath)
	}()

	if err := tmp.Chmod(0o600); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("chmod pickle key temp file: %w", err)
	}
	if _, err := tmp.WriteString(hex.EncodeToString(key) + "\n"); err != nil {
		_ = tmp.Close()
		return nil, fmt.Errorf("write pickle key temp file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		return nil, fmt.Errorf("close pickle key temp file: %w", err)
	}
	if err := os.Rename(tmpPath, path); err != nil {
		return nil, fmt.Errorf("persist pickle key: %w", err)
	}

	return key, nil
}

func parsePickleKey(data []byte, path string) ([]byte, error) {
	trimmed := strings.TrimSpace(string(data))
	if trimmed == "" {
		return nil, fmt.Errorf("pickle key file is empty: %s", path)
	}

	key, err := hex.DecodeString(trimmed)
	if err != nil {
		return nil, fmt.Errorf("decode pickle key: %w", err)
	}
	if len(key) != pickleKeySize {
		return nil, fmt.Errorf("pickle key at %s has %d bytes, want %d", path, len(key), pickleKeySize)
	}
	return key, nil
}
