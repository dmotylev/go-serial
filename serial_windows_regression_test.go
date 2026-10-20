//go:build windows

package serial

import (
	"context"
	"embed"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"
)

//go:embed testdata/windows/*/*.txt
var windowsRegressionFixtures embed.FS

func TestWindowsRegression(t *testing.T) {
	for _, family := range []string{"lifetime", "completion-order", "dcb"} {
		t.Run(family, func(t *testing.T) {
			sourceDir, err := os.Getwd()
			if err != nil {
				t.Fatal(err)
			}
			work := t.TempDir()
			entries, err := os.ReadDir(sourceDir)
			if err != nil {
				t.Fatal(err)
			}
			for _, entry := range entries {
				name := entry.Name()
				if entry.IsDir() || (name != "go.mod" && name != "go.sum" && (!strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go"))) {
					continue
				}
				data, err := os.ReadFile(filepath.Join(sourceDir, name))
				if err != nil {
					t.Fatal(err)
				}
				if name == "serial_windows.go" {
					source := string(data)
					if family == "completion-order" {
						gate := "\t\t\terr = windows.GetOverlappedResult("
						if strings.Count(source, gate) != 1 {
							t.Fatal("Read completion instrumentation site changed")
						}
						source = strings.Replace(source, gate, "\t\t\tprobeBeforeCompletion()\n"+gate, 1)
					}
					if family == "dcb" {
						field := "configMu   sync.Mutex"
						if strings.Count(source, field) != 1 {
							t.Fatal("configuration mutex instrumentation site changed")
						}
						source = strings.Replace(source, field, "configMu   probeConfigMutex", 1)
					}
					apis := []string{"CreateFile", "GetCommState", "SetCommState", "SetCommTimeouts", "CreateEvent", "CloseHandle", "ReadFile", "WriteFile", "GetOverlappedResult", "CancelIoEx"}
					if family == "lifetime" {
						apis = append(apis, "PurgeComm", "GetCommModemStatus", "FlushFileBuffers", "SetCommBreak", "ClearCommBreak")
					}
					for _, api := range apis {
						source = strings.ReplaceAll(source, "windows."+api+"(", "probe"+api+"(")
					}
					data = []byte(source)
				}
				if err := os.WriteFile(filepath.Join(work, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			for _, name := range []string{"probe_fake_windows.go", "probe_windows_test.go"} {
				data, err := windowsRegressionFixtures.ReadFile("testdata/windows/" + family + "/" + name + ".txt")
				if err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(work, name), data, 0o600); err != nil {
					t.Fatal(err)
				}
			}
			args := []string{"test", "-mod=readonly", "-vet=off", "-count=1", "-timeout=30s", "-v"}
			if windowsRegressionRace {
				args = append(args, "-race")
			}
			args = append(args, ".")
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
			defer cancel()
			cmd := exec.CommandContext(ctx, filepath.Join(runtime.GOROOT(), "bin", "go.exe"), args...)
			cmd.Dir = work
			cmd.Env = append(os.Environ(), "GOTOOLCHAIN=local", "GOPROXY=off", "GOSUMDB=off")
			output, err := cmd.CombinedOutput()
			t.Logf("current-source Windows %s regression (race=%t):\n%s", family, windowsRegressionRace, output)
			if err != nil {
				t.Fatalf("child regression: %v", err)
			}
		})
	}
}
