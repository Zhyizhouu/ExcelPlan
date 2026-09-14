// Command api serves ExcelPlan's progress endpoint against a local Obsidian
// vault. There is no database and no deploy target — this reads the same
// files Obsidian already treats as the source of truth, and is meant to run
// on the same machine as the vault.
package main

import (
	"log/slog"
	"net/http"
	"os"

	"github.com/Zhyizhouu/excelplan/internal/envcfg"
	"github.com/Zhyizhouu/excelplan/internal/httpapi"
	"github.com/Zhyizhouu/excelplan/internal/progresslog"
	"github.com/Zhyizhouu/excelplan/internal/tutor"
)

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Before any envcfg.Get call, so .env can supply any of the settings below.
	envcfg.Load(".env", logger)

	vaultRoot := envcfg.Get("EXCELPLAN_VAULT_ROOT",
		`D:\Abner\Obsidian\Fred\Excel Mastery`)
	hubNote := envcfg.Get("EXCELPLAN_HUB_NOTE",
		vaultRoot+`\Excel_Mastery.md`)
	logPath := envcfg.Get("EXCELPLAN_LOG_PATH", "progress-log.json")
	tutorPath := envcfg.Get("EXCELPLAN_TUTOR_LOG", "tutor-log.json")
	addr := envcfg.Get("EXCELPLAN_ADDR", ":8090")

	log, err := progresslog.Open(logPath)
	if err != nil {
		logger.Error("opening progress log", "path", logPath, "error", err)
		os.Exit(1)
	}

	tutorLog, err := tutor.OpenStore(tutorPath)
	if err != nil {
		logger.Error("opening tutor log", "path", tutorPath, "error", err)
		os.Exit(1)
	}

	// Nil when no key is set, which is a supported way to run: the plan, the
	// exports and the ticking all work, and only the tutor tab says it needs
	// configuring. A missing key must not stop the server starting.
	tutorClient := tutor.New(os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL"))
	if tutorClient.Configured() {
		logger.Info("tutor enabled", "model", tutorClient.Model())
	} else {
		logger.Warn("tutor disabled: set GEMINI_API_KEY in backend/.env to enable it")
	}

	server := httpapi.New(vaultRoot, hubNote, log, tutorClient, tutorLog, logger)
	logger.Info("listening", "addr", addr, "vault", vaultRoot)
	if err := http.ListenAndServe(addr, server.Handler()); err != nil {
		logger.Error("server stopped", "error", err)
		os.Exit(1)
	}
}
