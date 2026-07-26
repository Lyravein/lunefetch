package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/lyravein/lunefetch/internal/api"
	"github.com/lyravein/lunefetch/internal/config"
	"github.com/lyravein/lunefetch/internal/core"
	"github.com/lyravein/lunefetch/internal/storage"
	"github.com/lyravein/lunefetch/internal/ui"
)

func main() {
	url := flag.String("u", "", "URL to download")
	chunksOverride := flag.Int("c", 0, "Number of chunks (override)")
	output := flag.String("o", "", "Output directory")
	list := flag.Bool("list", false, "List all downloads")
	resumeID := flag.Int64("resume", 0, "Resume download by ID")
	flag.Parse()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	if *output != "" {
		cfg.DownloadDir = *output
	}

	dbPath := filepath.Join(os.Getenv("HOME"), ".local", "share", "lunefetch", "downloads.db")
	sm, err := storage.NewStateManager(dbPath)
	if err != nil {
		log.Fatalf("Failed to init database: %v", err)
	}
	defer sm.Close()

	if *list {
		listDownloads(sm)
		return
	}

	if *resumeID > 0 {
		resumeDownload(sm, cfg, *resumeID, *chunksOverride)
		return
	}

	if *url != "" {
		directDownload(sm, cfg, *url, *chunksOverride)
		return
	}

	m := ui.NewModel(sm, cfg)
	p := tea.NewProgram(m, tea.WithAltScreen())
	m.SetProgram(p)

	// Start local HTTP server so the browser extension can send URLs to the TUI.
	apiServer := api.New(api.DefaultAddr, func(url string) {
		p.Send(ui.AddURLMsg{URL: url})
	})
	if err := apiServer.Start(); err != nil {
		log.Printf("Warning: could not start API server on %s: %v", api.DefaultAddr, err)
		log.Printf("Browser extension integration will not be available.")
	} else {
		log.Printf("API server listening on %s", api.DefaultAddr)
	}
	defer apiServer.Stop()

	if _, err := p.Run(); err != nil {
		log.Fatalf("Error running program: %v", err)
	}
}

func directDownload(sm *storage.StateManager, cfg *config.Config, url string, chunksOverride int) {
	info, err := core.GetFileInfo(url)
	if err != nil {
		log.Fatalf("Failed to get file info: %v", err)
	}

	numChunks := cfg.ChunksForSize(info.Size)
	if chunksOverride > 0 {
		numChunks = chunksOverride
	}

	id, err := sm.CreateDownload(url, info.Filename, info.Size, info.SupportsRange, numChunks)
	if err != nil {
		log.Fatalf("Failed to create download: %v", err)
	}

	chunks := core.CalculateChunks(info.Size, numChunks)
	startBytes := make([]int64, len(chunks))
	endBytes := make([]int64, len(chunks))
	for i, c := range chunks {
		startBytes[i] = c.Start
		endBytes[i] = c.End
	}

	if err := sm.CreateChunks(id, startBytes, endBytes); err != nil {
		log.Fatalf("Failed to create chunks: %v", err)
	}

	ext := filepath.Ext(info.Filename)
	baseName := info.Filename[:len(info.Filename)-len(ext)]
	tmpPath := filepath.Join(cfg.DownloadDir, baseName+".tmp"+ext)

	downloader := core.NewDownloader(url, tmpPath, info.Size, chunks, numChunks, cfg.MaxRetries)
	downloader.SetProgressCallback(func(chunkIndex int, downloadedSize int64, status string) {
		sm.UpdateChunkProgress(id, chunkIndex, downloadedSize, status)
	})

	fmt.Printf("Downloading: %s (%s, %d chunks)\n", info.Filename, formatSizeCLI(info.Size), numChunks)

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				p := downloader.GetProgress()
				speed := downloader.GetSpeed()
				if p.TotalSize > 0 {
					pct := float64(p.DownloadedSize) / float64(p.TotalSize) * 100
					fmt.Printf("\rProgress: %.1f%% | Speed: %s | Active: %d",
						pct, formatSpeedCLI(speed), downloader.ActiveWorkers())
				}
			}
		}
	}()

	err = downloader.Start(context.Background())
	close(done)
	if err != nil {
		sm.UpdateDownloadStatus(id, "failed")
		fmt.Printf("\nDownload failed: %v\n", err)
		return
	}

	if err := core.MoveFile(tmpPath, filepath.Join(cfg.DownloadDir, info.Filename)); err != nil {
		sm.UpdateDownloadStatus(id, "failed")
		fmt.Printf("\nDownload finished but failed to move file: %v\n", err)
		return
	}
	sm.UpdateDownloadStatus(id, "completed")
	fmt.Println("\nDownload completed!")
}

func resumeDownload(sm *storage.StateManager, cfg *config.Config, id int64, chunksOverride int) {
	download, err := sm.GetDownload(id)
	if err != nil || download == nil {
		log.Fatalf("Download #%d not found", id)
	}

	chunks, err := sm.GetChunks(id)
	if err != nil {
		log.Fatalf("Failed to get chunks: %v", err)
	}

	chunkDefs := make([]core.Chunk, len(chunks))
	for i, c := range chunks {
		chunkDefs[i] = core.Chunk{
			Index:      c.ChunkIndex,
			Start:      c.StartByte,
			End:        c.EndByte,
			Downloaded: c.DownloadedSize,
		}
	}

	ext := filepath.Ext(download.Filename)
	baseName := download.Filename[:len(download.Filename)-len(ext)]
	tmpPath := filepath.Join(cfg.DownloadDir, baseName+".tmp"+ext)

	numChunks := download.NumChunks
	if chunksOverride > 0 {
		numChunks = chunksOverride
	}

	downloader := core.NewDownloader(download.URL, tmpPath, download.TotalSize, chunkDefs, numChunks, cfg.MaxRetries)
	downloader.SetProgressCallback(func(chunkIndex int, downloadedSize int64, status string) {
		sm.UpdateChunkProgress(id, chunkIndex, downloadedSize, status)
	})

	fmt.Printf("Resuming: %s (%s)\n", download.Filename, formatSizeCLI(download.TotalSize))

	done := make(chan struct{})
	go func() {
		ticker := time.NewTicker(1 * time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-done:
				return
			case <-ticker.C:
				p := downloader.GetProgress()
				speed := downloader.GetSpeed()
				if p.TotalSize > 0 {
					pct := float64(p.DownloadedSize) / float64(p.TotalSize) * 100
					fmt.Printf("\rProgress: %.1f%% | Speed: %s | Active: %d",
						pct, formatSpeedCLI(speed), downloader.ActiveWorkers())
				}
			}
		}
	}()

	err = downloader.Start(context.Background())
	close(done)
	if err != nil {
		sm.UpdateDownloadStatus(id, "failed")
		fmt.Printf("\nDownload failed: %v\n", err)
		return
	}

	if err := core.MoveFile(tmpPath, filepath.Join(cfg.DownloadDir, download.Filename)); err != nil {
		sm.UpdateDownloadStatus(id, "failed")
		fmt.Printf("\nDownload finished but failed to move file: %v\n", err)
		return
	}
	sm.UpdateDownloadStatus(id, "completed")
	fmt.Println("\nDownload completed!")
}

func listDownloads(sm *storage.StateManager) {
	downloads, err := sm.ListDownloads()
	if err != nil {
		log.Fatalf("Failed to list downloads: %v", err)
	}

	if len(downloads) == 0 {
		fmt.Println("No downloads found.")
		return
	}

	fmt.Printf("%-4s %-40s %-12s %-10s %s\n", "ID", "File", "Size", "Progress", "Status")
	fmt.Println(strings.Repeat("-", 80))
	for _, d := range downloads {
		progress := "0%"
		if d.TotalSize > 0 {
			progress = fmt.Sprintf("%.1f%%", float64(d.DownloadedSize)/float64(d.TotalSize)*100)
		}
		fmt.Printf("%-4d %-40s %-12s %-10s %s\n",
			d.ID, truncateString(d.Filename, 38), formatSizeCLI(d.TotalSize), progress, d.Status)
	}
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen-3] + "..."
}

func formatSizeCLI(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

func formatSpeedCLI(bytesPerSec float64) string {
	return formatSizeCLI(int64(bytesPerSec)) + "/s"
}
