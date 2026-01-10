package main

import (
	"embed"
	"log"
	"os"
	"path/filepath"
	"strconv"

	"github.com/lich0821/ccNexus/internal/storage"
	"github.com/wailsapp/wails/v2"
	"github.com/wailsapp/wails/v2/pkg/options"
	"github.com/wailsapp/wails/v2/pkg/options/assetserver"
	"github.com/wailsapp/wails/v2/pkg/options/mac"
	"github.com/wailsapp/wails/v2/pkg/options/windows"
)

//go:embed all:frontend/dist
var assets embed.FS

//go:embed build/windows/icon.ico
var trayIconWindows []byte

//go:embed build/appicon.png
var trayIconOther []byte

func main() {
	// Detect development mode by checking Wails environment or CCNEXUS_NO_PROXY
	// CCNEXUS_NO_PROXY: Use production database but disable proxy (for UI testing)
	// CCNEXUS_DEV_MODE: Use separate dev database (for isolated testing)
	isDevMode := os.Getenv("CCNEXUS_DEV_MODE") != ""
	isNoProxyMode := os.Getenv("CCNEXUS_NO_PROXY") != ""
	isWailsDev := os.Getenv("WAILS_ENVIRONMENT") == "development"

	if isNoProxyMode {
		log.Printf("Running in no-proxy mode (using production database)")
	} else if isDevMode {
		log.Printf("Running in development mode (using separate database)")
	} else if isWailsDev {
		log.Printf("Running in Wails development mode")
	}

	// Select appropriate tray icon based on OS
	var trayIcon []byte
	if os.PathSeparator == '\\' {
		// Windows
		trayIcon = trayIconWindows
	} else {
		// macOS, Linux, etc.
		trayIcon = trayIconOther
	}

	app := NewApp(trayIcon)

	// Load window size from SQLite storage
	windowWidth, windowHeight := 1024, 768 // defaults
	homeDir, err := os.UserHomeDir()
	if err == nil {
		// Use different config directory ONLY for CCNEXUS_DEV_MODE (separate database testing)
		// CCNEXUS_NO_PROXY uses production database
		configDirName := ".ccNexus"
		if isDevMode {
			configDirName = ".ccNexus-dev"
		}
		dbPath := filepath.Join(homeDir, configDirName, "ccnexus.db")
		if sqliteStorage, err := storage.NewSQLiteStorage(dbPath); err == nil {
			if w, err := sqliteStorage.GetConfig("windowWidth"); err == nil && w != "" {
				if width, err := strconv.Atoi(w); err == nil && width > 0 {
					windowWidth = width
				}
			}
			if h, err := sqliteStorage.GetConfig("windowHeight"); err == nil && h != "" {
				if height, err := strconv.Atoi(h); err == nil && height > 0 {
					windowHeight = height
				}
			}
			sqliteStorage.Close()
		}
	}

	err = wails.Run(&options.App{
		Title:       "ccNexus",
		Width:       windowWidth,
		Height:      windowHeight,
		StartHidden: false,
		AssetServer: &assetserver.Options{
			Assets: assets,
		},
		BackgroundColour: &options.RGBA{R: 255, G: 255, B: 255, A: 255},
		OnStartup:        app.startup,
		OnShutdown:       app.shutdown,
		OnBeforeClose:    app.beforeClose,
		Bind: []interface{}{
			app,
		},
		Frameless:     false,
		Fullscreen:    false,
		MinWidth:      800,
		MinHeight:     600,
		DisableResize: false,
		Mac: &mac.Options{
			TitleBar: &mac.TitleBar{
				TitlebarAppearsTransparent: false,
				HideTitle:                  false,
				HideTitleBar:               false,
				FullSizeContent:            false,
				UseToolbar:                 false,
				HideToolbarSeparator:       false,
			},
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			About: &mac.AboutInfo{
				Title:   "ccNexus",
				Message: "© 2024 ccNexus\n\nA smart API endpoint rotation proxy for Claude Code",
			},
		},
		Windows: &windows.Options{
			WebviewIsTransparent: false,
			WindowIsTranslucent:  false,
			DisableWindowIcon:    false,
		},
	})

	if err != nil {
		log.Fatal(err)
	}
}
