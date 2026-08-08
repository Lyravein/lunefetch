package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/lyravein/lunefetch/internal/api"
	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui/components"
	"github.com/lyravein/lunefetch/internal/ui/layout"
)

var version = "dev"

func main() {
	if len(os.Args) == 2 && os.Args[1] == "--version" {
		fmt.Printf("lunefetch %s\n", version)
		return
	}

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	dbPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "lunefetch", "downloads.db")
	sm, err := storage.NewStateManager(dbPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
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
		log.Fatalf("Failed to initialize local API authentication: %v", err)
	}
	apiServer := api.New(api.DefaultAddr, apiToken, func(req api.DownloadRequest) bool {
		select {
		case guiApp.AddURLCh <- components.AddURLRequest{URL: req.URL, Filename: req.Filename, SaveDir: req.SaveDir}:
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
