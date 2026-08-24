package singleinstance

import (
	"errors"
	"os"
	"path/filepath"
	"strconv"
	"strings"
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

// The pid must stay readable while the lock is held. Windows byte-range locks
// are mandatory, so locking byte 0 made the file unreadable even to the owner;
// the lock now sits on a sentinel byte past any content.
func TestAcquireCreatesLockFileWithPid(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "nested", "state")

	lock, err := Acquire(dir)
	if err != nil {
		t.Fatalf("Acquire: %v", err)
	}
	defer lock.Release()

	data, err := os.ReadFile(filepath.Join(dir, "lunefetch.lock"))
	if err != nil {
		t.Fatalf("read lock file while held: %v", err)
	}
	pid, err := strconv.Atoi(strings.TrimSpace(string(data)))
	if err != nil {
		t.Fatalf("lock file content = %q, want the owning pid: %v", data, err)
	}
	if pid != os.Getpid() {
		t.Fatalf("lock file pid = %d, want %d", pid, os.Getpid())
	}
}
