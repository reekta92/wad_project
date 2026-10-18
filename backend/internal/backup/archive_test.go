package backup

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func client(t *testing.T, program string) {
	t.Helper()
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "pg_dump"), []byte("#!/bin/sh\nset -eu\n"+program), 0700); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", directory+":/usr/bin:/bin")
}

func TestArchiveIsCompletedProtectedAndCleanedAfterDelivery(t *testing.T) {
	client(t, `test -z "${EXAM_OWNER_PASSWORD:-}"
test -z "${KEYCLOAK_DB_PASSWORD:-}"
test -z "${PGPASSWORD:-}"
case "$PGDATABASE" in *private*) exit 11;; esac
test "$(stat -c %a "$PGPASSFILE")" = 600
test "$(stat -c %a "$(dirname "$PGPASSFILE")")" = 700
grep -q 'private' "$PGPASSFILE"
for arg in "$@"; do case "$arg" in *postgres*|*private*) exit 12;; esac; done
printf 'PGDMPcompleted archive'
`)
	t.Setenv("EXAM_OWNER_PASSWORD", "must-not-reach-child")
	t.Setenv("KEYCLOAK_DB_PASSWORD", "must-not-reach-child")
	service := New("postgres://exam_admin:private@localhost:5433/exams?sslmode=disable")
	archive, err := service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	directory := archive.directory
	stat, err := archive.Stat()
	if err != nil || stat.Mode().Perm() != 0600 {
		t.Fatalf("unprotected archive: %v %v", stat, err)
	}
	if _, err := os.Stat(filepath.Join(directory, "pgpass")); !os.IsNotExist(err) {
		t.Fatal("credential file retained after dump")
	}
	body, err := io.ReadAll(archive)
	if err != nil || string(body) != "PGDMPcompleted archive" {
		t.Fatalf("incomplete archive: %q %v", body, err)
	}
	if _, err := service.Create(context.Background()); err != ErrBusy {
		t.Fatal("concurrent archive accepted")
	}
	if err := archive.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(directory); !os.IsNotExist(err) {
		t.Fatal("temporary archive retained")
	}
	next, err := service.Create(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	next.Close()
}

func TestFailedAndTimedOutDumpsReleaseTheirSlot(t *testing.T) {
	for _, program := range []string{"printf 'PGDMPpartial'; exit 1", "printf 'not an archive'", "exec sleep 10"} {
		client(t, program)
		service := New("postgres://exam_admin:private@localhost/exams?sslmode=disable")
		for range 2 {
			ctx, cancel := context.WithTimeout(context.Background(), 25*time.Millisecond)
			archive, err := service.Create(ctx)
			cancel()
			if err != ErrUnavailable || archive != nil {
				t.Fatalf("failed dump was delivered: %v %v", archive, err)
			}
		}
	}
}

func TestArchiveOutputLimitStopsTheWriter(t *testing.T) {
	var output bytes.Buffer
	writer := boundedWriter{target: &output, remaining: 3}
	if _, err := writer.Write([]byte("ab")); err != nil {
		t.Fatal(err)
	}
	if _, err := writer.Write([]byte("cd")); err != ErrUnavailable || strings.Contains(output.String(), "c") {
		t.Fatal("partial oversized write accepted")
	}
}
