package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/lyravein/lunefetch/internal/api"
	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/singleinstance"
	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/components"
	"github.com/lyravein/lunefetch/internal/ui/fatal"
	"github.com/lyravein/lunefetch/internal/ui/layout"
	"github.com/lyravein/lunefetch/internal/userpath"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("lunefetch %s\n", version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		fatal.Show("Configuration error", fmt.Sprintf("Lunefetch could not read its configuration file.\n\n%v", err))
	}

	// A second instance would share this user's SQLite handle (limited to one
	// connection) and write the same .part files, so refuse to start rather than
	// corrupt in-flight downloads.
	lock, err := singleinstance.Acquire(userpath.Data())
	if err != nil {
		if errors.Is(err, singleinstance.ErrAlreadyRunning) {
			fatal.Show("Lunefetch is already running",
				"Another Lunefetch window is already open for this user.\n\n"+
					"Use the Lunefetch icon in your system tray to bring it back, "+
					"or quit it from the tray menu before starting a new one.")
		}
		fatal.Show("Startup error", fmt.Sprintf("Lunefetch could not claim its single-instance lock.\n\n%v", err))
	}
	defer lock.Release()

	dbPath := filepath.Join(userpath.Data(), "downloads.db")
	sm, err := storage.NewStateManager(dbPath)
	if err != nil {
		fatal.Show("Database error", fmt.Sprintf("Lunefetch could not open its download database.\n\n%v", err))
	}
	defer sm.Close()

	// Auto-purge history yang sudah melewati batas retensi.
	if cfg.HistoryRetentionDays > 0 {
		cutoff := time.Now().AddDate(0, 0, -cfg.HistoryRetentionDays)
		sm.PurgeOlderThan(cutoff) //nolint:errcheck
	}

	guiApp := layout.New(sm, cfg)
	guiApp.RecoverDownloads()

	// Scheduler: cek setiap menit apakah ada download yang jadwalnya sudah tiba.
	go func() {
		for {
			now := time.Now()
			hhmm := fmt.Sprintf("%02d:%02d", now.Hour(), now.Minute())
			if ready, err := sm.ListScheduledDue(hhmm); err == nil {
				for _, d := range ready {
					id := d.ID
					sm.UpdateDownloadStatus(id, "pending") //nolint:errcheck
					sm.SetScheduledAt(id, nil)             //nolint:errcheck
					guiApp.EnqueueScheduled(id)
				}
			}
			next := now.Truncate(time.Minute).Add(time.Minute)
			timer := time.NewTimer(time.Until(next))
			select {
			case <-guiApp.Done():
				timer.Stop()
				return
			case <-timer.C:
			}
		}
	}()

	// Start HTTP server untuk browser extension.
	apiToken, err := api.LoadOrCreateToken()
	if err != nil {
		fatal.Show("Authentication error", fmt.Sprintf("Lunefetch could not initialize the local API token used by the browser extension.\n\n%v", err))
	}
	apiServer := api.New(api.DefaultAddr, apiToken, func(req api.DownloadRequest) bool {
		// The extension is not a trusted source of filesystem paths. Confine any
		// destination hint to the configured download directory so a compromised
		// or malicious caller cannot create files elsewhere (e.g. autostart).
		saveDir, err := core.SafeSaveDir(cfg.DownloadDir, req.SaveDir)
		if err != nil {
			log.Printf("Rejected API destination hint: %v", err)
			return false
		}
		select {
		case guiApp.AddURLCh <- components.AddURLRequest{URL: req.URL, Filename: req.Filename, SaveDir: saveDir}:
			return true
		default:
			return false
		}
	})
	if err := apiServer.Start(); err != nil {
		log.Printf("Warning: could not start API server: %v", err)
	}
	defer apiServer.Stop()

	guiApp.Run()
}
