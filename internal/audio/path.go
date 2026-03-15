package audio

import (
	"errors"
	"path/filepath"
	"strings"
)

// ResolveLibraryPath joins a Navidrome-reported track path to the configured mounted library root.
func ResolveLibraryPath(root, navPath string) (string, error) {
	if strings.TrimSpace(root) == "" {
		return "", errors.New("library root is required")
	}
	cleanRoot := filepath.Clean(root)
	cleanNavPath := strings.TrimLeft(strings.TrimSpace(navPath), `/\`)
	if cleanNavPath == "" {
		return "", errors.New("track path is required")
	}
	resolved := filepath.Join(cleanRoot, cleanNavPath)
	rel, err := filepath.Rel(cleanRoot, resolved)
	if err != nil {
		return "", err
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", errors.New("track path escapes library root")
	}
	return resolved, nil
}
