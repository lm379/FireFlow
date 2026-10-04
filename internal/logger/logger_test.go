package logger

import (
	"bytes"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/gin-gonic/gin"
)

func TestConsoleOnlyLoggingDoesNotCreateDirectory(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dir := filepath.Join(t.TempDir(), "logs")
	if err := initWithConfig(Config{Level: "warn"}, dir, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Close() })
	Printf("filtered message")
	Warn("warning message")
	if stdout.Len() != 0 || !strings.Contains(stderr.String(), "warning message") {
		t.Fatalf("incorrect level/output: stdout=%s stderr=%s", &stdout, &stderr)
	}
	if _, err := os.Stat(dir); !os.IsNotExist(err) {
		t.Fatalf("console-only logging created a directory: %v", err)
	}
	if n, err := GetGinLogWriter().Write([]byte("disabled HTTP log")); err != nil || n == 0 {
		t.Fatalf("disabled writer failed: %d %v", n, err)
	}
	if stdout.Len() != 0 {
		t.Fatal("disabled HTTP log reached stdout")
	}
}

func TestGinMiddlewareHonorsLevelsAndOmitsQuery(t *testing.T) {
	gin.SetMode(gin.TestMode)
	var stdout, stderr bytes.Buffer
	config := Config{Level: "error", EnableGinLogger: true}
	if err := initWithConfig(config, t.TempDir(), &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Close() })
	router := gin.New()
	router.Use(GinMiddleware())
	router.GET("/ok", func(c *gin.Context) { c.Status(200) })
	router.GET("/failure", func(c *gin.Context) { c.Status(500) })
	for _, path := range []string{"/ok", "/failure?token=private-token"} {
		w := httptest.NewRecorder()
		router.ServeHTTP(w, httptest.NewRequest("GET", path, nil))
	}
	output := stdout.String()
	if strings.Contains(output, "/ok") || strings.Contains(output, "private-token") || !strings.Contains(output, "/failure") || !strings.Contains(output, "500") {
		t.Fatalf("unexpected access log: %s", output)
	}
}

func TestFileOutputUsesConfiguredRotationAndReinitializes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	dir := t.TempDir()
	config := Config{Level: "info", EnableFileOutput: true, EnableGinLogger: true, MaxFileSize: 3, MaxBackups: 2, MaxAge: 5, Compress: true}
	if err := initWithConfig(config, dir, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { Close() })
	if len(fileOutputs) != 3 {
		t.Fatalf("expected three rotating outputs, got %d", len(fileOutputs))
	}
	for _, output := range fileOutputs {
		if output.MaxSize != 3 || output.MaxBackups != 2 || output.MaxAge != 5 || !output.Compress {
			t.Fatalf("rotation settings lost: %#v", output)
		}
	}
	Printf("application message")
	Error("error message")
	if _, err := GetGinLogWriter().Write([]byte("request message\n")); err != nil {
		t.Fatal(err)
	}
	day := time.Now().Format("2006-01-02")
	for file, message := range map[string]string{"app-" + day + ".log": "application message", "error-" + day + ".log": "error message", "gin-" + day + ".log": "request message"} {
		data, err := os.ReadFile(filepath.Join(dir, file))
		if err != nil || !strings.Contains(string(data), message) {
			t.Fatalf("%s: data=%s err=%v", file, data, err)
		}
	}
	config.EnableFileOutput = false
	config.EnableGinLogger = false
	if err := initWithConfig(config, dir, &stdout, &stderr); err != nil {
		t.Fatal(err)
	}
	if len(fileOutputs) != 0 {
		t.Fatal("old rotating outputs retained after reinitialization")
	}
	Printf("console message")
	data, err := os.ReadFile(filepath.Join(dir, "app-"+day+".log"))
	if err != nil || strings.Contains(string(data), "console message") {
		t.Fatalf("file output remained enabled: %s, %v", data, err)
	}
}
