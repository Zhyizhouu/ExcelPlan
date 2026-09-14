// Package envcfg is the shared .env/environment-variable loader for
// ExcelPlan's two entrypoints (the dev API server and the packaged desktop
// app). Both need the same "env var wins over .env file, .env is optional"
// behavior, so it lives here instead of being copied twice.
package envcfg

import (
	"bufio"
	"log/slog"
	"os"
	"strings"
)

// Get returns the environment variable's value, or fallback if unset.
func Get(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Load reads KEY=value lines from path into the environment.
//
// Hand-rolled rather than pulled in as a dependency: this has one job and
// the format it needs is four lines of parsing. The backend has no
// third-party dependencies beyond excelize, and an API-key loader is a poor
// place to start adding them.
//
// A real environment variable always wins over the file, so a value exported
// in the shell overrides .env rather than being silently ignored — the
// opposite is the sort of thing that costs an afternoon.
func Load(path string, logger *slog.Logger) {
	f, err := os.Open(path)
	if err != nil {
		return // no .env is normal; every setting has a default or is optional
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		key, value, found := strings.Cut(line, "=")
		if !found {
			continue
		}
		key = strings.TrimSpace(key)
		// Quotes are stripped because a key pasted from a web page often
		// arrives wrapped in them, and "AIza..." is not a valid key.
		value = strings.Trim(strings.TrimSpace(value), `"'`)
		if key == "" || value == "" {
			continue
		}
		if _, already := os.LookupEnv(key); already {
			continue
		}
		if err := os.Setenv(key, value); err != nil {
			logger.Warn("setting env from .env", "key", key, "error", err)
		}
	}
}
