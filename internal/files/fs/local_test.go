package fs

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLocalFSListAndReadText(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "hello.txt")
	if err := os.WriteFile(filePath, []byte("hello"), 0o644); err != nil {
		t.Fatal(err)
	}

	local := NewLocalFS()
	items, total, err := local.List(root, ListOptions{Page: 1, Size: 10, SortBy: "name", SortOrder: "asc"})
	if err != nil {
		t.Fatal(err)
	}
	if total != 1 || len(items) != 1 || items[0].Name != "hello.txt" {
		t.Fatalf("List() = total %d items %#v", total, items)
	}

	content, err := local.ReadText(filePath, 1024)
	if err != nil {
		t.Fatal(err)
	}
	if content != "hello" {
		t.Fatalf("ReadText() = %q", content)
	}
}

func TestLocalFSRejectsBinaryRead(t *testing.T) {
	root := t.TempDir()
	filePath := filepath.Join(root, "bin.dat")
	if err := os.WriteFile(filePath, []byte{0, 1, 2, 3}, 0o644); err != nil {
		t.Fatal(err)
	}

	local := NewLocalFS()
	if _, err := local.ReadText(filePath, 1024); err != ErrBinary {
		t.Fatalf("ReadText() error = %v, want ErrBinary", err)
	}
}
