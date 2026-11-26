package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"

	"golang.org/x/term"
	"taeu.kr/cohesion/internal/account"
	accountStore "taeu.kr/cohesion/internal/account/store"
	"taeu.kr/cohesion/internal/config"
	"taeu.kr/cohesion/internal/platform/database"
)

const adminCommandUsage = "Usage: cohesion admin reset-password"

type recoveryTerminal interface {
	io.Writer
	ReadLine() (string, error)
	ReadPassword(prompt string) (string, error)
}

func runAdminCommand(args []string, input, output *os.File) error {
	if len(args) != 1 || args[0] != "reset-password" {
		return errors.New(adminCommandUsage)
	}
	fd := int(input.Fd())
	if !term.IsTerminal(fd) || !term.IsTerminal(int(output.Fd())) {
		return errors.New("password recovery requires an interactive terminal")
	}

	terminalState, err := term.MakeRaw(fd)
	if err != nil {
		return fmt.Errorf("open recovery terminal: %w", err)
	}
	defer term.Restore(fd, terminalState)

	terminal := term.NewTerminal(struct {
		io.Reader
		io.Writer
	}{input, output}, "Administrator username: ")
	if width, height, err := term.GetSize(int(output.Fd())); err == nil && width > 0 && height > 0 {
		_ = terminal.SetSize(width, height)
	}
	_, _ = fmt.Fprintln(terminal, "Reset a local administrator password. Press Ctrl+C to cancel.")
	return resetAdministratorPassword(goEnv, terminal)
}

func resetAdministratorPassword(environment string, terminal recoveryTerminal) error {
	cfg, configPath, err := config.ReadExistingConfig(environment)
	if err != nil {
		return fmt.Errorf("locate the existing Cohesion installation: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	db, databasePath, err := database.OpenExisting(ctx, cfg.Datasource.URL, filepath.Dir(configPath))
	cancel()
	if err != nil {
		return fmt.Errorf("locate the existing Cohesion database: %w", err)
	}
	defer db.Close()

	_, _ = fmt.Fprintf(terminal, "Database: %q\n", databasePath)
	store := accountStore.NewStore(db)
	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	admins, err := store.ListAdministratorUsernames(ctx)
	cancel()
	if err != nil {
		return fmt.Errorf("list existing administrators: %w", err)
	}
	switch len(admins) {
	case 0:
		return errors.New("no administrator account exists in this installation")
	case 1:
		_, _ = fmt.Fprintf(terminal, "Only administrator: %q. Press Enter to select this account.\n", admins[0])
	default:
		_, _ = fmt.Fprintln(terminal, "Select an administrator by username:")
		for _, admin := range admins {
			_, _ = fmt.Fprintf(terminal, "  %q\n", admin)
		}
	}
	username, err := terminal.ReadLine()
	if err != nil && !errors.Is(err, term.ErrPasteIndicator) {
		return errors.New("password reset cancelled")
	}
	username = strings.TrimSpace(username)
	if username == "" {
		if len(admins) != 1 {
			return errors.New("enter the username of the administrator you want to recover")
		}
		username = admins[0]
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	target, err := store.GetUserByUsername(ctx, username)
	cancel()
	if err != nil {
		return fmt.Errorf("find administrator: %w", err)
	}
	if target.Role != account.RoleAdmin {
		return fmt.Errorf("account %q is not an administrator", target.Username)
	}

	_, _ = fmt.Fprintf(terminal, "Administrator: %q\n", target.Username)
	password, err := terminal.ReadPassword("New password: ")
	if err != nil && !errors.Is(err, term.ErrPasteIndicator) {
		return errors.New("password reset cancelled")
	}
	confirmedPassword, err := terminal.ReadPassword("Confirm new password: ")
	if err != nil && !errors.Is(err, term.ErrPasteIndicator) {
		return errors.New("password reset cancelled")
	}
	if password != confirmedPassword {
		return errors.New("passwords do not match")
	}

	ctx, cancel = context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := account.NewService(store).ResetAdminPassword(ctx, target, password); err != nil {
		return fmt.Errorf("reset administrator password: %w", err)
	}
	_, _ = fmt.Fprintf(terminal, "Password reset for %q. Sign in with your new password.\n", target.Username)
	return nil
}
