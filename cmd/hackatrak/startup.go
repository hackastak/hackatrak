package main

import (
	"context"
	"fmt"

	"github.com/spf13/cobra"

	"github.com/hackastak/hackatrak/internal/config"
	"github.com/hackastak/hackatrak/internal/store"
)

type appKey struct{}

type appState struct {
	cfg config.Config
	db  *store.DB
}

// prepare loads config and opens the database for every command except version.
// Version must run even when config is missing. A bad config fails before the
// database is created, so the first successful run is what migrates the schema.
func prepare(cmd *cobra.Command, _ []string) error {
	switch cmd.Name() {
	case "version", "install", "uninstall":
		return nil
	}

	configPath, err := config.DefaultPath()
	if err != nil {
		return err
	}
	cfg, err := config.Load(configPath)
	if err != nil {
		return err
	}

	dbPath, err := store.DefaultPath()
	if err != nil {
		return err
	}
	db, err := store.Open(dbPath)
	if err != nil {
		return err
	}

	ctx := cmd.Context()
	if ctx == nil {
		ctx = context.Background()
	}
	cmd.SetContext(context.WithValue(ctx, appKey{}, appState{cfg: cfg, db: db}))
	return nil
}

func appFrom(cmd *cobra.Command) (appState, error) {
	ctx := cmd.Context()
	if ctx == nil {
		return appState{}, fmt.Errorf("app state missing")
	}
	state, ok := ctx.Value(appKey{}).(appState)
	if !ok || state.db == nil {
		return appState{}, fmt.Errorf("app state missing")
	}
	return state, nil
}

func finish(cmd *cobra.Command, _ []string) error {
	ctx := cmd.Context()
	if ctx == nil {
		return nil
	}
	state, ok := ctx.Value(appKey{}).(appState)
	if !ok || state.db == nil {
		return nil
	}
	if err := state.db.Close(); err != nil {
		return fmt.Errorf("shutdown: %w", err)
	}
	return nil
}
