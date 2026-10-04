package logger

import (
	"bufio"
	"compress/gzip"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"sync"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"
)

type dailyWriter struct {
	*lumberjack.Logger
	mu               sync.Mutex
	dir, prefix, day string
	config           Config
	stop, done       chan struct{}
	closeOnce        sync.Once
	closeErr         error
}

func newDailyWriter(dir, filename string, config Config) (*dailyWriter, error) {
	w := &dailyWriter{dir: dir, prefix: strings.TrimSuffix(filename, ".log"), config: config, stop: make(chan struct{}), done: make(chan struct{})}
	if err := w.advance(time.Now()); err != nil {
		return nil, err
	}
	go func() {
		defer close(w.done)
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-w.stop:
				return
			case now := <-ticker.C:
				w.mu.Lock()
				err := w.advance(now)
				w.mu.Unlock()
				if err != nil {
					fmt.Fprintf(os.Stderr, "日志保留清理失败: %v\n", err)
				}
			}
		}
	}()
	return w, nil
}

// advance 必须持有写入锁；启动、跨日写入和定时巡检都会执行清理。
func (w *dailyWriter) advance(now time.Time) error {
	day := now.Format("2006-01-02")
	if day == w.day {
		return nil
	}
	if w.Logger != nil {
		if err := w.Logger.Close(); err != nil {
			return err
		}
	}
	if err := w.cleanup(now); err != nil {
		return err
	}
	w.Logger = &lumberjack.Logger{Filename: filepath.Join(w.dir, w.prefix+"-"+day+".log"), MaxSize: w.config.MaxFileSize, MaxBackups: w.config.MaxBackups, MaxAge: w.config.MaxAge, Compress: w.config.Compress, LocalTime: true}
	w.day = day
	return nil
}

func (w *dailyWriter) Write(p []byte) (int, error) {
	w.mu.Lock()
	defer w.mu.Unlock()
	select {
	case <-w.stop:
		return 0, os.ErrClosed
	default:
	}
	if err := w.advance(time.Now()); err != nil {
		return 0, err
	}
	return w.Logger.Write(p)
}

func (w *dailyWriter) Close() error {
	w.closeOnce.Do(func() {
		close(w.stop)
		<-w.done
		w.mu.Lock()
		defer w.mu.Unlock()
		w.closeErr = w.Logger.Close()
	})
	return w.closeErr
}

func (w *dailyWriter) cleanup(now time.Time) error {
	cutoff := now.AddDate(0, 0, 1-w.config.MaxAge).Format("2006-01-02")
	entries, err := os.ReadDir(w.dir)
	if err != nil {
		return err
	}
	daily := regexp.MustCompile(`^` + regexp.QuoteMeta(w.prefix) + `-(\d{4}-\d{2}-\d{2})(?:-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{3})?\.log(?:\.gz)?$`)
	legacy := regexp.MustCompile(`^` + regexp.QuoteMeta(w.prefix) + `(?:-\d{4}-\d{2}-\d{2}T\d{2}-\d{2}-\d{2}\.\d{3})?\.log(?:\.gz)?$`)
	for _, entry := range entries {
		if !entry.Type().IsRegular() {
			continue
		}
		path := filepath.Join(w.dir, entry.Name())
		if match := daily.FindStringSubmatch(entry.Name()); match != nil {
			if _, err := time.Parse("2006-01-02", match[1]); err != nil {
				continue
			}
			if match[1] < cutoff {
				if err := os.Remove(path); err != nil {
					return err
				}
			}
		} else if legacy.MatchString(entry.Name()) {
			if err := trimLegacyLog(path, cutoff); err != nil {
				return err
			}
		}
	}
	return nil
}

var logDate = regexp.MustCompile(`^(?:time="|\[GIN\]\s*)?(\d{4}[-/]\d{2}[-/]\d{2})`)

// trimLegacyLog 按记录时间清理旧版本混合多天的文件；续行跟随上一条记录。
func trimLegacyLog(path, cutoff string) error {
	source, err := os.Open(path)
	if err != nil {
		return err
	}
	defer source.Close()
	info, err := source.Stat()
	if err != nil {
		return err
	}
	reader := io.Reader(source)
	compressed := strings.HasSuffix(path, ".gz")
	if compressed {
		z, err := gzip.NewReader(source)
		if err != nil {
			return err
		}
		defer z.Close()
		reader = z
	}
	temp, err := os.CreateTemp(filepath.Dir(path), ".retention-*")
	if err != nil {
		return err
	}
	defer func() { temp.Close(); os.Remove(temp.Name()) }()
	writer := io.Writer(temp)
	var zipped *gzip.Writer
	if compressed {
		zipped = gzip.NewWriter(temp)
		writer = zipped
	}
	date := info.ModTime().Format("2006-01-02")
	scanner := bufio.NewScanner(reader)
	scanner.Buffer(make([]byte, 64*1024), 16*1024*1024)
	removed, kept := false, false
	for scanner.Scan() {
		line := scanner.Text()
		if match := logDate.FindStringSubmatch(line); match != nil {
			date = strings.ReplaceAll(match[1], "/", "-")
		}
		if date < cutoff {
			removed = true
			continue
		}
		if _, err := fmt.Fprintln(writer, line); err != nil {
			return err
		}
		kept = true
	}
	if err := scanner.Err(); err != nil {
		return err
	}
	if zipped != nil {
		if err := zipped.Close(); err != nil {
			return err
		}
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if !removed {
		return nil
	}
	if err := source.Close(); err != nil {
		return err
	}
	if !kept {
		return os.Remove(path)
	}
	if err := os.Chmod(temp.Name(), info.Mode().Perm()); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
