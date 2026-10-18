// Package backup creates bounded custom-format application database archives.
package backup

import (
	"context"
	"errors"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
)

const MediaType = "application/octet-stream"
const maxBytes = 256 * 1024 * 1024

var ErrBusy = errors.New("a backup is already in progress")
var ErrUnavailable = errors.New("backup could not be completed")

type Service struct {
	databaseURL string
	running     chan struct{}
}

func New(databaseURL string) *Service {
	return &Service{databaseURL: databaseURL, running: make(chan struct{}, 1)}
}

type Archive struct {
	*os.File
	directory string
	release   func()
}

func (a *Archive) Close() error {
	err := a.File.Close()
	os.RemoveAll(a.directory)
	if a.release != nil {
		a.release()
		a.release = nil
	}
	return err
}

func (s *Service) Create(ctx context.Context) (_ *Archive, failure error) {
	select {
	case s.running <- struct{}{}:
	default:
		return nil, ErrBusy
	}
	defer func() {
		if failure != nil {
			<-s.running
		}
	}()
	connection, err := pgx.ParseConfig(s.databaseURL)
	if err != nil {
		return nil, ErrUnavailable
	}
	uri, err := url.Parse(s.databaseURL)
	if err != nil || (uri.Scheme != "postgres" && uri.Scheme != "postgresql") {
		return nil, ErrUnavailable
	}
	password := connection.Password
	if uri.User != nil {
		uri.User = url.User(uri.User.Username())
	}
	query := uri.Query()
	query.Del("password")
	query.Del("passfile")
	query.Del("sslpassword")
	uri.RawQuery = query.Encode()
	directory, err := os.MkdirTemp("", "wad-backup-")
	if err != nil {
		return nil, ErrUnavailable
	}
	defer func() {
		if failure != nil {
			os.RemoveAll(directory)
		}
	}()
	var fields []string
	for _, field := range []string{connection.Host, strconv.Itoa(int(connection.Port)), connection.Database, connection.User, password} {
		if strings.ContainsAny(field, "\r\n") {
			return nil, ErrUnavailable
		}
		fields = append(fields, strings.NewReplacer(`\`, `\\`, ":", `\:`).Replace(field))
	}
	passfile := filepath.Join(directory, "pgpass")
	if err := os.WriteFile(passfile, []byte(strings.Join(fields, ":")+"\n"), 0600); err != nil {
		return nil, ErrUnavailable
	}
	file, err := os.OpenFile(filepath.Join(directory, "application.dump"), os.O_CREATE|os.O_EXCL|os.O_RDWR, 0600)
	if err != nil {
		return nil, ErrUnavailable
	}
	defer func() {
		if failure != nil {
			file.Close()
		}
	}()
	// libpq expands URI dbnames passed as arguments; PGDATABASE defaults do not expand them.
	// Only the credential-free URI is an argument; pgpass and child environment remain private.
	command := exec.CommandContext(ctx, "pg_dump", "--format=custom", "--no-password", "--enable-row-security", "--lock-wait-timeout=5000", "--dbname="+uri.String())
	command.Env = []string{"PATH=" + os.Getenv("PATH"), "LANG=C.UTF-8", "PGPASSFILE=" + passfile}
	command.Stdout = &boundedWriter{target: file, remaining: maxBytes}
	command.Stderr = io.Discard
	if err := command.Run(); err != nil {
		return nil, ErrUnavailable
	}
	var signature [5]byte
	if _, err := file.ReadAt(signature[:], 0); err != nil || string(signature[:]) != "PGDMP" {
		return nil, ErrUnavailable
	}
	if _, err := file.Seek(0, io.SeekStart); err != nil {
		return nil, ErrUnavailable
	}
	os.Remove(passfile)
	return &Archive{File: file, directory: directory, release: func() { <-s.running }}, nil
}

type boundedWriter struct {
	target    io.Writer
	remaining int
}

func (w *boundedWriter) Write(data []byte) (int, error) {
	if len(data) > w.remaining {
		return 0, ErrUnavailable
	}
	n, err := w.target.Write(data)
	w.remaining -= n
	return n, err
}
