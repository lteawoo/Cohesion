package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"
)

type updaterArgs struct {
	pid         int
	target      string
	replacement string
	workdir     string
	argsFile    string
	cleanupDir  string
}

type updaterLogger struct {
	out io.Writer
}

func (l updaterLogger) logf(format string, args ...any) {
	_, _ = fmt.Fprintf(l.out, "[updater] %s\n", fmt.Sprintf(format, args...))
}

func openRootLogFile(targetPath, fileName string) (*os.File, string, error) {
	targetDir := filepath.Dir(filepath.Clean(targetPath))
	logsDir := filepath.Join(targetDir, "logs")
	if err := os.MkdirAll(logsDir, 0o755); err != nil {
		return nil, "", err
	}

	logPath := filepath.Join(logsDir, fileName)
	logFile, err := os.OpenFile(logPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, "", err
	}
	return logFile, logPath, nil
}

func main() {
	args, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[updater] invalid arguments: %v\n", err)
		os.Exit(2)
	}

	logger := updaterLogger{out: os.Stderr}
	logFile, updaterLogPath, logErr := openRootLogFile(args.target, "updater.log")
	if logErr != nil {
		fmt.Fprintf(os.Stderr, "[updater] failed to initialize updater.log: %v\n", logErr)
	} else {
		defer logFile.Close()
		logger.out = io.MultiWriter(os.Stderr, logFile)
		logger.logf("updater log initialized: %s", updaterLogPath)
	}

	appLogPath := filepath.Join(filepath.Dir(args.target), "logs", "app.log")
	logger.logf("starting update flow (pid=%d, target=%s, replacement=%s)", args.pid, args.target, args.replacement)
	if err := run(args, appLogPath, logger); err != nil {
		logger.logf("failed: %v", err)
		os.Exit(1)
	}
	logger.logf("completed successfully")
}

func parseFlags() (updaterArgs, error) {
	parsed := updaterArgs{}

	flag.IntVar(&parsed.pid, "pid", 0, "target process pid")
	flag.StringVar(&parsed.target, "target", "", "current executable path")
	flag.StringVar(&parsed.replacement, "replacement", "", "new executable path")
	flag.StringVar(&parsed.workdir, "workdir", "", "working directory for restart")
	flag.StringVar(&parsed.argsFile, "args-file", "", "json file path for app arguments")
	flag.StringVar(&parsed.cleanupDir, "cleanup-dir", "", "temporary directory to cleanup")
	flag.Parse()

	if parsed.pid <= 0 {
		return updaterArgs{}, errors.New("pid is required")
	}
	if strings.TrimSpace(parsed.target) == "" {
		return updaterArgs{}, errors.New("target is required")
	}
	if strings.TrimSpace(parsed.replacement) == "" {
		return updaterArgs{}, errors.New("replacement is required")
	}
	if strings.TrimSpace(parsed.argsFile) == "" {
		return updaterArgs{}, errors.New("args-file is required")
	}

	parsed.target = filepath.Clean(parsed.target)
	parsed.replacement = filepath.Clean(parsed.replacement)
	if strings.TrimSpace(parsed.workdir) == "" {
		parsed.workdir = filepath.Dir(parsed.target)
	}
	parsed.workdir = filepath.Clean(parsed.workdir)

	return parsed, nil
}

func run(args updaterArgs, appLogPath string, logger updaterLogger) error {
	if args.cleanupDir != "" {
		defer os.RemoveAll(args.cleanupDir)
	}

	appArgs, err := readAppArgs(args.argsFile)
	if err != nil {
		return err
	}

	backupPath := args.target + ".bak"
	if err := waitForProcessExit(args.pid, 90*time.Second); err != nil {
		return err
	}

	if err := replaceWithRetry(args.target, args.replacement, backupPath, 30*time.Second); err != nil {
		return err
	}

	if err := restartApplication(args.target, args.workdir, appArgs, appLogPath, logger); err != nil {
		_ = rollbackBinary(args.target, backupPath)
		return err
	}

	return nil
}

func readAppArgs(argsFilePath string) ([]string, error) {
	payload, err := os.ReadFile(argsFilePath)
	if err != nil {
		return nil, err
	}

	var args []string
	if err := json.Unmarshal(payload, &args); err != nil {
		return nil, err
	}
	return args, nil
}

func replaceWithRetry(targetPath, replacementPath, backupPath string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for {
		err := replaceBinary(targetPath, replacementPath, backupPath)
		if err == nil {
			return nil
		}

		if time.Now().After(deadline) {
			return err
		}
		time.Sleep(500 * time.Millisecond)
	}
}

func replaceBinary(targetPath, replacementPath, backupPath string) error {
	if _, err := os.Stat(replacementPath); err != nil {
		return fmt.Errorf("replacement binary is not available: %w", err)
	}

	_ = os.Remove(backupPath)

	if err := os.Rename(targetPath, backupPath); err != nil {
		return fmt.Errorf("failed to backup current binary: %w", err)
	}

	if err := os.Rename(replacementPath, targetPath); err != nil {
		_ = os.Rename(backupPath, targetPath)
		return fmt.Errorf("failed to apply replacement binary: %w", err)
	}

	if err := os.Chmod(targetPath, 0o755); err != nil {
		return fmt.Errorf("failed to set executable permission: %w", err)
	}

	return nil
}

func rollbackBinary(targetPath, backupPath string) error {
	if _, err := os.Stat(backupPath); err != nil {
		return err
	}
	_ = os.Remove(targetPath)
	return os.Rename(backupPath, targetPath)
}

func restartApplication(targetPath, workdir string, appArgs []string, appLogPath string, logger updaterLogger) error {
	cmd := exec.Command(targetPath, appArgs...)
	cmd.Dir = workdir
	appLogFile, err := openAppLogFile(appLogPath)
	if err != nil {
		logger.logf("failed to open app log file (%s), fallback to stdio: %v", appLogPath, err)
		cmd.Stdout = os.Stdout
		cmd.Stderr = os.Stderr
	} else {
		defer appLogFile.Close()
		cmd.Stdout = appLogFile
		cmd.Stderr = appLogFile
		logger.logf("restarted app logs redirected to: %s", appLogPath)
	}
	cmd.Stdin = nil
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to restart app: %w", err)
	}

	return nil
}

func openAppLogFile(appLogPath string) (*os.File, error) {
	if err := os.MkdirAll(filepath.Dir(appLogPath), 0o755); err != nil {
		return nil, err
	}
	return os.OpenFile(appLogPath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
}
