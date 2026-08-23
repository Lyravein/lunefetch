package singleinstance

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestAcquireRejectsSecondHolder(t *testing.T) {
	dir := t.TempDir()

	first, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	defer first.Release()

	if _, err := Acquire(dir); !errors.Is(err, ErrAlreadyRunning) {
		t.Fatalf("second Acquire error = %v, want ErrAlreadyRunning", err)
	}
}

func TestLockIsReusableAfterRelease(t *testing.T) {
	dir := t.TempDir()

	first, err := Acquire(dir)
	if err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	first.Release()
	first.Release() // must be idempotent

	second, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire after Release: %v", err)
	}
	second.Release()
}

func TestAcquireCreatesLockFileWithPid(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state")

	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer lock.Release()

	data, err := os.ReadFile(filepath.Join(dir, "lunefetch.lock"))
	if err != nil {
		t.Fatalf("read lock file: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("lock file is empty, want the owning pid for diagnostics")
	}
}
