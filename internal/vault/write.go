package vault

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

const writeAttempts = 3

// ErrNoteChanged means the note kept changing between the read used to splice
// and the re-read immediately before rename. The destination is left as the
// other writer left it.
var ErrNoteChanged = errors.New("vault: note changed while writing")

// WriteAtomic re-reads path, applies splice to that fresh content, then
// re-reads again immediately before replacing the file. If the file changed
// during splice, it retries with the newer content. The replacement is a temp
// file in the same directory plus rename, so a crash cannot truncate the note.
// Equal content is not rewritten. splice must be pure; an error leaves the
// file untouched.
func WriteAtomic(path string, splice func(note string) (string, error)) error {
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("vault: note path is empty")
	}
	if splice == nil {
		return fmt.Errorf("vault: splice func is nil")
	}
	for attempt := 0; attempt < writeAttempts; attempt++ {
		data, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("vault: reread note: %w", err)
		}
		next, err := splice(string(data))
		if err != nil {
			return err
		}
		latest, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("vault: reread note: %w", err)
		}
		if !bytes.Equal(latest, data) {
			continue
		}
		if bytes.Equal(latest, []byte(next)) {
			return nil
		}
		return replaceFile(path, []byte(next))
	}
	return fmt.Errorf("%w: %s", ErrNoteChanged, path)
}

func replaceFile(path string, data []byte) error {
	info, err := os.Stat(path)
	if err != nil {
		return fmt.Errorf("vault: stat note: %w", err)
	}
	dir := filepath.Dir(path)
	f, err := os.CreateTemp(dir, ".hackatrak-*.tmp")
	if err != nil {
		return fmt.Errorf("vault: create temp note: %w", err)
	}
	tmp := f.Name()
	success := false
	defer func() {
		if !success {
			_ = os.Remove(tmp)
		}
	}()
	if err := f.Chmod(info.Mode().Perm()); err != nil {
		_ = f.Close()
		return fmt.Errorf("vault: chmod temp note: %w", err)
	}
	if _, err := f.Write(data); err != nil {
		_ = f.Close()
		return fmt.Errorf("vault: write temp note: %w", err)
	}
	if err := f.Sync(); err != nil {
		_ = f.Close()
		return fmt.Errorf("vault: sync temp note: %w", err)
	}
	if err := f.Close(); err != nil {
		return fmt.Errorf("vault: close temp note: %w", err)
	}
	if err := os.Rename(tmp, path); err != nil {
		return fmt.Errorf("vault: rename temp note: %w", err)
	}
	success = true
	syncDir(dir)
	return nil
}

func syncDir(dir string) {
	f, err := os.Open(dir)
	if err != nil {
		return
	}
	defer f.Close()
	_ = f.Sync()
}
