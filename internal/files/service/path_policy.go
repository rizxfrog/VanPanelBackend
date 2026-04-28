package service

import (
	"os"
	"path/filepath"
	"strings"
)

type PathPolicy struct {
	config ManagerConfig
}

func NewPathPolicy(config ManagerConfig) *PathPolicy {
	return &PathPolicy{config: config}
}

func (p *PathPolicy) Resolve(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", ErrFilePathDenied
	}

	abs, err := filepath.Abs(filepath.Clean(input))
	if err != nil {
		return "", err
	}

	resolved := abs
	evaluated, err := filepath.EvalSymlinks(abs)
	if err == nil {
		resolved = evaluated
	} else if !os.IsNotExist(err) {
		return "", err
	}

	resolved = filepath.Clean(resolved)
	if p.config.AllowFullDisk {
		return resolved, nil
	}

	for _, root := range p.config.Roots {
		if isWithinRoot(resolved, filepath.Clean(root.Path)) {
			return resolved, nil
		}
	}
	return "", ErrFilePathDenied
}

func (p *PathPolicy) ResolveParentForCreate(input string) (string, error) {
	if strings.TrimSpace(input) == "" {
		return "", ErrFilePathDenied
	}
	parent := filepath.Dir(filepath.Clean(input))
	if _, err := p.Resolve(parent); err != nil {
		return "", err
	}
	abs, err := filepath.Abs(filepath.Clean(input))
	if err != nil {
		return "", err
	}
	return filepath.Clean(abs), nil
}

func isWithinRoot(pathValue, root string) bool {
	if pathValue == root {
		return true
	}
	rel, err := filepath.Rel(root, pathValue)
	if err != nil {
		return false
	}
	return rel != "." && rel != ".." && !strings.HasPrefix(rel, ".."+string(filepath.Separator))
}
