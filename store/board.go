package store

import (
	"bufio"
	"bytes"
	"crypto/rand"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/gofrs/flock"
)

const (
	fileRoster     = "roster.jsonl"
	fileNotices    = "notices.jsonl"
	fileChallenges = "challenges.jsonl"
	fileCheckins   = "checkins.jsonl"
	fileWrites     = "writes.jsonl"
	fileLock       = ".lock"
)

// Board is a flock-guarded JSONL corkboard.
type Board struct {
	dir  string
	id   Identity
	now  func() time.Time
	mu   sync.Mutex
	lock *flock.Flock
}

// Open creates dir if needed and takes a lock file next to the JSONL.
func Open(cfg Config) (*Board, error) {
	if cfg.Identity.Author == "" || !authorRE.MatchString(cfg.Identity.Author) {
		return nil, teach("BOARDS_AUTHOR must be a short slug (kit, maya).", `set BOARDS_AUTHOR=kit`)
	}
	if cfg.Path == "" {
		return nil, teach("BOARDS_PATH is required.", `set BOARDS_PATH=/boards`)
	}
	if cfg.Identity.WritesPerDay < 1 {
		cfg.Identity.WritesPerDay = defaultAgentWrites
	}
	if err := os.MkdirAll(cfg.Path, 0o755); err != nil {
		return nil, fmt.Errorf("mkdir board: %w", err)
	}
	now := cfg.Now
	if now == nil {
		now = time.Now
	}
	return &Board{
		dir:  cfg.Path,
		id:   cfg.Identity,
		now:  now,
		lock: flock.New(file(cfg.Path, fileLock)),
	}, nil
}

// Author is this process's crane slug.
func (b *Board) Author() string { return b.id.Author }

func (b *Board) do(fn func() error) error {
	b.mu.Lock()
	defer b.mu.Unlock()
	if err := b.lock.Lock(); err != nil {
		return fmt.Errorf("board lock: %w", err)
	}
	defer func() { _ = b.lock.Unlock() }()
	return fn()
}

func readJSONL[T any](path string) ([]T, error) {
	raw, err := os.ReadFile(path) //nolint:gosec // G304: path is BOARDS_PATH + known JSONL filename
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}
	var out []T
	sc := bufio.NewScanner(bytes.NewReader(raw))
	// notices can be 500 chars; keep a modest buffer.
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 1024*1024)
	for sc.Scan() {
		line := bytes.TrimSpace(sc.Bytes())
		if len(line) == 0 {
			continue
		}
		var row T
		if err := json.Unmarshal(line, &row); err != nil {
			return nil, fmt.Errorf("jsonl %s: %w", path, err)
		}
		out = append(out, row)
	}
	return out, sc.Err()
}

func writeJSONL[T any](path string, rows []T) error {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	for _, row := range rows {
		if err := enc.Encode(row); err != nil {
			return err
		}
	}
	return os.WriteFile(path, buf.Bytes(), 0o644) //nolint:gosec // G306: board JSONL is not secret
}

func appendJSONL[T any](path string, row T) error {
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644) //nolint:gosec // G302: board JSONL is not secret
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(row)
}

func newID(prefix string, now time.Time) string {
	var n uint32
	if err := binary.Read(rand.Reader, binary.BigEndian, &n); err != nil {
		n = uint32(now.UnixNano()) //nolint:gosec // fallback uniqueness
	}
	return prefix + strconv36(now.UnixNano()) + strconv36(int64(n))
}

func strconv36(n int64) string {
	const digits = "0123456789abcdefghijklmnopqrstuvwxyz"
	if n == 0 {
		return "0"
	}
	if n < 0 {
		n = -n
	}
	var b [16]byte
	i := len(b)
	for n > 0 && i > 0 {
		i--
		b[i] = digits[n%36]
		n /= 36
	}
	return string(b[i:])
}
