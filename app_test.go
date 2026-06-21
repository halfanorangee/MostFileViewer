package main

import (
	"os"
	"path/filepath"
	"testing"
)

func TestValidateFilePathAllowsSelectedFileOutsideCurrentRoot(t *testing.T) {
	root := t.TempDir()
	external := t.TempDir()

	rootFile := filepath.Join(root, "root.txt")
	if err := os.WriteFile(rootFile, []byte("root"), 0o644); err != nil {
		t.Fatalf("write root file: %v", err)
	}

	externalFile := filepath.Join(external, "external.txt")
	if err := os.WriteFile(externalFile, []byte("external"), 0o644); err != nil {
		t.Fatalf("write external file: %v", err)
	}

	app := NewApp()
	if _, err := app.LoadFolderTree(root); err != nil {
		t.Fatalf("load root folder: %v", err)
	}

	if _, _, err := app.validateFilePath(externalFile); err == nil {
		t.Fatal("expected external file to be rejected before it is explicitly allowed")
	}

	app.allowFile(externalFile)
	if _, _, err := app.validateFilePath(externalFile); err != nil {
		t.Fatalf("expected explicitly allowed external file to be readable: %v", err)
	}

	if _, err := app.LoadFolderChildren(root); err != nil {
		t.Fatalf("expected original root to remain active after allowing external file: %v", err)
	}
}

func TestLoadFolderTreeClearsAllowedFiles(t *testing.T) {
	firstRoot := t.TempDir()
	secondRoot := t.TempDir()
	external := t.TempDir()
	externalFile := filepath.Join(external, "external.txt")
	if err := os.WriteFile(externalFile, []byte("external"), 0o644); err != nil {
		t.Fatalf("write external file: %v", err)
	}

	app := NewApp()
	if _, err := app.LoadFolderTree(firstRoot); err != nil {
		t.Fatalf("load first root folder: %v", err)
	}
	app.allowFile(externalFile)
	if _, _, err := app.validateFilePath(externalFile); err != nil {
		t.Fatalf("expected external file to be readable after allow: %v", err)
	}

	if _, err := app.LoadFolderTree(secondRoot); err != nil {
		t.Fatalf("load second root folder: %v", err)
	}
	if _, _, err := app.validateFilePath(externalFile); err == nil {
		t.Fatal("expected allowed external file to be rejected after switching folders")
	}
}
