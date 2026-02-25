package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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

func main() {
	args, err := parseFlags()
	if err != nil {
		fmt.Fprintf(os.Stderr, "[updater] invalid arguments: %v\n", err)
		os.Exit(2)
	}

	if err := run(args); err != nil {
		fmt.Fprintf(os.Stderr, "[updater] failed: %v\n", err)
		os.Exit(1)
	}
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

func run(args updaterArgs) error {
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

	if err := restartApplication(args.target, args.workdir, appArgs); err != nil {
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

func restartApplication(targetPath, workdir string, appArgs []string) error {
	cmd := exec.Command(targetPath, appArgs...)
	cmd.Dir = workdir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	cmd.Stdin = nil
	cmd.Env = os.Environ()

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to restart app: %w", err)
	}

	return nil
}
