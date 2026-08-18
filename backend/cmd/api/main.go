// Command api serves ExcelPlan's progress endpoint against a local Obsidian
// vault. There is no database and no deploy target — this reads the same
// files Obsidian already treats as the source of truth, and is meant to run
// on the same machine as the vault.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/Zhyizhouu/excelplan/internal/httpapi"
	"github.com/Zhyizhouu/excelplan/internal/progresslog"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	vaultRoot := env("EXCELPLAN_VAULT_ROOT",
		`D:\Abner\Obsidian\Fred\Excel Mastery`)
	hubNote := env("EXCELPLAN_HUB_NOTE",
		vaultRoot+`\Excel_Mastery.md`)
	logPath := env("EXCELPLAN_LOG_PATH", "progress-log.json")
	addr := env("EXCELPLAN_ADDR", ":8090")

	log, err := progresslog.Open(logPath)
	if err != nil {
		logger.Error("opening progress log", "path", logPath, "error", err)
		os.Exit(1)
	}

	server := httpapi.New(vaultRoot, hubNote, log, logger)
	logger.Info("listening", "addr", addr, "vault", vaultRoot)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
