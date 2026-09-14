// Command desktop packages ExcelPlan as a native Windows window: the same
// httpapi server as cmd/api, plus the production frontend build embedded
// into the binary and shown in a chromeless WebView2 window via Wails.
//
// It reads and writes the same vault, progress log, and tutor log as the dev
// server in backend/ — this is a second front door onto one shared state,
// not a separate instance. Paths are hardcoded absolute, matching cmd/api's
// existing convention for a single-machine, single-user tool.
package main

import (
	"context"
	"embed"
	"io/fs"
	"log/slog"
	"net/http"
	"os"

	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/windows"

	"github.com/Zhyizhouu/excelplan/internal/envcfg"
	"github.com/Zhyizhouu/excelplan/internal/httpapi"
	"github.com/Zhyizhouu/excelplan/internal/progresslog"
	"github.com/Zhyizhouu/excelplan/internal/tutor"
)

//go:embed all:frontend_dist
var embeddedAssets embed.FS

// assets strips the frontend_dist/ prefix so index.html lands at the root
// Wails' asset server expects, instead of at frontend_dist/index.html.
func assets() fs.FS {
	sub, err := fs.Sub(embeddedAssets, "frontend_dist")
	if err != nil {
		panic(err) // frontend_dist is embedded above; this can't fail at runtime
	}
	return sub
}

const backendDir = `D:\Abner\!Programs\excelplan\backend`

func main() {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))

	// Shares backend/.env with cmd/api rather than a separate copy, so the
	// Gemini key only lives in one place.
	envcfg.Load(backendDir+`\.env`, logger)

	vaultRoot := envcfg.Get("EXCELPLAN_VAULT_ROOT",
		`D:\Abner\Obsidian\Fred\Excel Mastery`)
	hubNote := envcfg.Get("EXCELPLAN_HUB_NOTE",
		vaultRoot+`\Excel_Mastery.md`)
	logPath := envcfg.Get("EXCELPLAN_LOG_PATH", backendDir+`\progress-log.json`)
	tutorPath := envcfg.Get("EXCELPLAN_TUTOR_LOG", backendDir+`\tutor-log.json`)
	addr := envcfg.Get("EXCELPLAN_ADDR", "127.0.0.1:8090")

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

	tutorClient := tutor.New(os.Getenv("GEMINI_API_KEY"), os.Getenv("GEMINI_MODEL"))
	if tutorClient.Configured() {
		logger.Info("tutor enabled", "model", tutorClient.Model())
	} else {
		logger.Warn("tutor disabled: set GEMINI_API_KEY in backend/.env to enable it")
	}

	server := httpapi.New(vaultRoot, hubNote, log, tutorClient, tutorLog, logger)

	// The API keeps running on its own port; the window only shows the
	// frontend build. The frontend already calls http://localhost:8090
	// directly (see frontend/src/api), so no proxying is needed here.
	go func() {
		logger.Info("api listening", "addr", addr, "vault", vaultRoot)
		if err := http.ListenAndServe(addr, server.Handler()); err != nil {
			logger.Error("api server stopped", "error", err)
		}
	}()

	err = wails.Run(&options.App{
		Title:            "ExcelPlan",
		Width:            1280,
		Height:           800,
		MinWidth:         900,
		MinHeight:        600,
		Assets:           assets(),
		BackgroundColour: &options.RGBA{R: 15, G: 15, B: 15, A: 1},
		OnStartup:        func(ctx context.Context) {},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
		},
	})
	if err != nil {
		logger.Error("wails run", "error", err)
		os.Exit(1)
	}
}
