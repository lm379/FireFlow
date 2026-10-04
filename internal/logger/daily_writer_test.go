package logger

import (
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestDailyRetentionCleansAllLogFiles(t *testing.T) {
	for _, prefix := range []string{"app", "error", "gin"} {
		t.Run(prefix, func(t *testing.T) {
			dir := t.TempDir()
			now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
			for _, name := range []string{prefix + "-2026-10-07.log", prefix + "-2026-10-07-2026-10-08T00-00-00.000.log.gz", prefix + "-2026-10-08.log", prefix + "-2026-10-14.log", "unrelated.log", prefix + "-invalid.log", "other-2026-10-01.log"} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			w := &dailyWriter{dir: dir, prefix: prefix, config: Config{MaxAge: 7, MaxFileSize: 1}}
			if err := w.advance(now); err != nil {
				t.Fatal(err)
			}
			defer func() { w.Logger.Close() }()
			for _, name := range []string{prefix + "-2026-10-07.log", prefix + "-2026-10-07-2026-10-08T00-00-00.000.log.gz"} {
				if _, err := os.Stat(filepath.Join(dir, name)); !os.IsNotExist(err) {
					t.Fatalf("expired log retained: %s", name)
				}
			}
			for _, name := range []string{prefix + "-2026-10-08.log", prefix + "-2026-10-14.log", "unrelated.log", prefix + "-invalid.log", "other-2026-10-01.log"} {
				if _, err := os.Stat(filepath.Join(dir, name)); err != nil {
					t.Fatalf("retained file removed: %s", name)
				}
			}
			if _, err := w.Logger.Write([]byte("day fourteen\n")); err != nil {
				t.Fatal(err)
			}
			if err := w.advance(now.AddDate(0, 0, 1)); err != nil {
				t.Fatal(err)
			}
			if _, err := os.Stat(filepath.Join(dir, prefix+"-2026-10-08.log")); !os.IsNotExist(err) {
				t.Fatal("idle boundary cleanup did not remove expired day")
			}
			if _, err := w.Logger.Write([]byte("day fifteen\n")); err != nil {
				t.Fatal(err)
			}
			data, err := os.ReadFile(filepath.Join(dir, prefix+"-2026-10-15.log"))
			if err != nil || string(data) != "day fifteen\n" {
				t.Fatalf("cross-day output mixed: %s %v", data, err)
			}
		})
	}
}

func TestLegacyRetentionUsesRecordDate(t *testing.T) {
	for _, format := range []string{"app", "gin", "standard"} {
		for _, compressed := range []bool{false, true} {
			t.Run(format+map[bool]string{false: " plain", true: " gzip"}[compressed], func(t *testing.T) {
				prefixes := map[string][]string{"app": {`time="2026-10-07 12:00:00 CST"`, `time="2026-10-08 00:00:00 CST"`}, "gin": {"[GIN] 2026/10/07 - 12:00:00", "[GIN] 2026/10/08 - 00:00:00"}, "standard": {"2026/10/07 12:00:00", "2026/10/08 00:00:00"}}
				lines := prefixes[format]
				content := lines[0] + " old-record\nold continuation\n" + lines[1] + " retained-record\nretained continuation\n"
				path := filepath.Join(t.TempDir(), "app.log")
				if compressed {
					path += ".gz"
				}
				f, err := os.Create(path)
				if err != nil {
					t.Fatal(err)
				}
				if compressed {
					z := gzip.NewWriter(f)
					_, err = z.Write([]byte(content))
					if err != nil {
						t.Fatal(err)
					}
					if err := z.Close(); err != nil {
						t.Fatal(err)
					}
				} else {
					if _, err := f.WriteString(content); err != nil {
						t.Fatal(err)
					}
				}
				f.Close()
				if err := trimLegacyLog(path, "2026-10-08"); err != nil {
					t.Fatal(err)
				}
				f, err = os.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				defer f.Close()
				var reader io.Reader = f
				if compressed {
					z, err := gzip.NewReader(f)
					if err != nil {
						t.Fatal(err)
					}
					defer z.Close()
					reader = z
				}
				data, err := io.ReadAll(reader)
				if err != nil || strings.Contains(string(data), "old") || !strings.Contains(string(data), "retained-record\nretained continuation") {
					t.Fatalf("incorrect filtered records: %s %v", data, err)
				}
			})
		}
	}
}

func TestDailyWriterCloseStopsCleanup(t *testing.T) {
	w, err := newDailyWriter(t.TempDir(), "app.log", Config{MaxAge: 7, MaxFileSize: 1})
	if err != nil {
		t.Fatal(err)
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	select {
	case <-w.done:
	default:
		t.Fatal("cleanup goroutine still running")
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("after close")); err == nil {
		t.Fatal("closed writer accepted output")
	}
}

func TestRetentionUsesConfiguredNumberOfDays(t *testing.T) {
	for _, days := range []int{1, 7, 10, 30} {
		t.Run(fmt.Sprint(days), func(t *testing.T) {
			dir := t.TempDir()
			now := time.Date(2026, 10, 14, 12, 0, 0, 0, time.FixedZone("CST", 8*3600))
			retained := "app-" + now.AddDate(0, 0, 1-days).Format("2006-01-02") + ".log"
			expired := "app-" + now.AddDate(0, 0, -days).Format("2006-01-02") + ".log"
			for _, name := range []string{retained, expired} {
				if err := os.WriteFile(filepath.Join(dir, name), []byte("fixture"), 0600); err != nil {
					t.Fatal(err)
				}
			}
			w := &dailyWriter{dir: dir, prefix: "app", config: Config{MaxAge: days, MaxFileSize: 1}}
			if err := w.advance(now); err != nil {
				t.Fatal(err)
			}
			defer w.Logger.Close()
			if _, err := os.Stat(filepath.Join(dir, retained)); err != nil {
				t.Fatalf("%d-day boundary removed: %v", days, err)
			}
			if _, err := os.Stat(filepath.Join(dir, expired)); !os.IsNotExist(err) {
				t.Fatalf("%d-day expired file retained: %v", days, err)
			}
		})
	}
}
