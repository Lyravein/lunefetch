package ui

import (
	"fmt"
	"strings"
	"time"

	"github.com/lyravein/lunefetch/internal/storage"
)

func (m *model) View() string {
	switch m.currentPage {
	case pageList:
		return m.listView()
	case pageDetail:
		return m.detailView()
	case pageAddURL:
		return m.addURLView()
	case pageSchedule:
		return m.scheduleView()
	case pageConflict:
		return m.conflictView()
	case pageDuplicate:
		return m.duplicateView()
	case pageRename:
		return m.renameView()
	case pageSetFolder:
		return m.setFolderView()
	case pageSpeedLimit:
		return m.speedLimitView()
	}
	return ""
}

func (m *model) listView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" Lunefetch "))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
		m.err = nil
	}

	b.WriteString(m.table.View())
	b.WriteString("\n\n")

	// Tampilkan speed limit aktif kalau ada yang di-set.
	if m.globalLimit > 0 || len(m.itemLimits) > 0 {
		parts := []string{}
		if m.globalLimit > 0 {
			parts = append(parts, "global "+formatSpeedLimit(m.globalLimit))
		}
		if n := len(m.itemLimits); n > 0 {
			parts = append(parts, fmt.Sprintf("%d file dibatasi", n))
		}
		b.WriteString(helpStyle.Render("  Limit: " + strings.Join(parts, "  |  ")))
		b.WriteString("\n")
	}

	b.WriteString(helpStyle.Render(
		"  n: New  |  r: Resume  |  p: Pause  |  d: Delete  |  s: Schedule  |  x: Unschedule  |  Shift+↑↓: Reorder  |  enter: Detail  |  q: Quit",
	))
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  l: Speed Limit"))

	return b.String()
}

func (m *model) detailView() string {
	ad, ok := m.activeDownloads[m.selectedID]
	if !ok {
		download, err := m.state.GetDownload(m.selectedID)
		if err != nil || download == nil {
			return "Download not found.\n\nPress esc to go back."
		}
		return m.downloadInfoView(download)
	}

	return m.activeDownloadView(ad)
}

func (m *model) downloadInfoView(d *storage.DownloadRecord) string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(fmt.Sprintf(" Download #%d ", d.ID)))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("File:   %s\n", d.Filename))
	b.WriteString(fmt.Sprintf("Size:   %s\n", formatSize(d.TotalSize)))
	b.WriteString(fmt.Sprintf("Status: %s\n", d.Status))

	if d.ScheduledAt.Valid {
		b.WriteString(fmt.Sprintf("Scheduled: %s\n", d.ScheduledAt.String))
	}

	if d.TotalSize > 0 {
		pct := float64(d.DownloadedSize) / float64(d.TotalSize) * 100
		b.WriteString(fmt.Sprintf("Progress: %.1f%%\n", pct))
	}

	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  esc: Back  |  q: Quit"))

	return b.String()
}

func (m *model) activeDownloadView(ad *activeDownload) string {
	var b strings.Builder
	p := ad.downloader.GetProgress()
	speed := ad.downloader.GetSpeed()

	b.WriteString(titleStyle.Render(fmt.Sprintf(" Download #%d ", ad.state.ID)))
	b.WriteString("\n\n")

	b.WriteString(fmt.Sprintf("File: %s\n", ad.state.Filename))
	b.WriteString(fmt.Sprintf("Size: %s\n", formatSize(p.TotalSize)))
	b.WriteString(fmt.Sprintf("Speed: %s\n", formatSpeed(speed)))

	if p.TotalSize > 0 {
		pct := float64(p.DownloadedSize) / float64(p.TotalSize) * 100
		elapsed := time.Since(p.StartTime)
		eta := time.Duration(0)
		if speed > 0 {
			remaining := float64(p.TotalSize-p.DownloadedSize) / speed
			eta = time.Duration(remaining) * time.Second
		}

		b.WriteString(fmt.Sprintf("Progress: %.1f%%\n", pct))
		b.WriteString(fmt.Sprintf("Elapsed: %s\n", formatDuration(elapsed)))
		b.WriteString(fmt.Sprintf("ETA: %s\n", formatDuration(eta)))

		barWidth := m.width - 6 // 2 for brackets, 4 for padding
		if barWidth < 10 {
			barWidth = 10
		}
		if barWidth > 80 {
			barWidth = 80
		}
		filled := int(float64(barWidth) * float64(p.DownloadedSize) / float64(p.TotalSize))
		if filled > barWidth {
			filled = barWidth
		}
		bar := strings.Repeat("█", filled) + strings.Repeat("░", barWidth-filled)
		b.WriteString(fmt.Sprintf("[%s]\n", bar))
	}

	b.WriteString("\nChunks:\n")
	for _, cp := range p.Chunks {
		chunkPct := float64(0)
		if cp.TotalSize > 0 {
			chunkPct = float64(cp.DownloadedSize) / float64(cp.TotalSize) * 100
		}

		statusChar := "⏳"
		switch cp.Status {
		case "completed":
			statusChar = "✅"
		case "failed":
			statusChar = "❌"
		case "retrying":
			statusChar = "🔄"
		case "downloading":
			statusChar = "⬇️"
		}

		chunkBarWidth := m.width/4 - 6
		if chunkBarWidth < 10 {
			chunkBarWidth = 10
		}
		if chunkBarWidth > 30 {
			chunkBarWidth = 30
		}
		chunkFilled := int(float64(chunkBarWidth) * chunkPct / 100)
		if chunkFilled > chunkBarWidth {
			chunkFilled = chunkBarWidth
		}
		chunkBar := strings.Repeat("▓", chunkFilled) + strings.Repeat("▒", chunkBarWidth-chunkFilled)

		b.WriteString(fmt.Sprintf("  Chunk #%d %s [%s] %.1f%%\n",
			cp.ChunkIndex, statusChar, chunkBar, chunkPct))
	}

	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  esc: Back  |  q: Quit"))

	return b.String()
}

func (m *model) addURLView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" New Download "))
	b.WriteString("\n\n")
	b.WriteString("Enter URL:\n")
	b.WriteString(m.urlInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  enter: Confirm  |  esc: Cancel"))

	return b.String()
}

func (m *model) renameView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" Rename File "))
	b.WriteString("\n\n")
	b.WriteString("Nama file (kosong = nama asli):\n")
	b.WriteString(m.renameInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  enter: Lanjut  |  esc: Lewati"))

	return b.String()
}

func (m *model) setFolderView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" Pilih Folder Tujuan "))
	b.WriteString("\n\n")
	b.WriteString("Folder tujuan:\n")
	b.WriteString(m.folderInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  enter: Mulai Download  |  esc: Batal"))

	return b.String()
}

func (m *model) speedLimitView() string {
	var b strings.Builder

	b.WriteString(titleStyle.Render(" Speed Limit "))
	b.WriteString("\n\n")

	if m.err != nil {
		b.WriteString(errorStyle.Render(fmt.Sprintf("Error: %v", m.err)))
		b.WriteString("\n\n")
		m.err = nil
	}

	// Tampilkan kedua scope sekaligus, tandai yang sedang diedit, supaya jelas
	// mana yang akan tersimpan saat enter.
	globalMark, selMark := "  ", "> "
	if m.speedScope == scopeGlobal {
		globalMark, selMark = "> ", "  "
	}

	b.WriteString(globalMark + "Global   " + formatSpeedLimit(m.globalLimit) + "\n")
	b.WriteString(helpStyle.Render("           total bandwidth semua download"))
	b.WriteString("\n")

	if m.hasSpeedTarget() {
		b.WriteString(selMark + "File     " + formatSpeedLimit(m.itemLimits[m.speedTargetID]) + "\n")
		b.WriteString(helpStyle.Render("           hanya " + truncate(m.speedTargetName, 40)))
	} else {
		b.WriteString(helpStyle.Render("  File     (pilih download dulu di list)"))
		b.WriteString("\n")
		b.WriteString(helpStyle.Render("           hanya berlaku untuk yang belum selesai"))
	}
	b.WriteString("\n\n")

	if m.speedScope == scopeGlobal {
		b.WriteString("Limit global baru:\n")
	} else {
		b.WriteString("Limit untuk " + truncate(m.speedTargetName, 40) + ":\n")
	}
	b.WriteString(m.speedInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  Format: 500k, 2m, 1.5m, 1g  |  kosong = unlimited"))
	b.WriteString("\n")
	if m.hasSpeedTarget() {
		b.WriteString(helpStyle.Render("  tab: Global / File  |  enter: Simpan  |  esc: Batal"))
	} else {
		b.WriteString(helpStyle.Render("  enter: Simpan  |  esc: Batal"))
	}

	return b.String()
}

// truncate memendekkan s ke maksimal n karakter, menambahkan elipsis kalau
// terpotong, supaya nama file panjang tidak merusak layout.
func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	if n <= 1 {
		return string(r[:n])
	}
	return string(r[:n-1]) + "…"
}

func (m *model) duplicateView() string {
	if m.duplicate == nil {
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(" URL Sudah Ada "))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("URL ini sudah pernah didownload sebagai:\n  %q\n\n", m.duplicate.existingName))
	b.WriteString("Pilih aksi:\n\n")
	b.WriteString("  A  Add anyway  — download ulang dengan nama baru (#1, #2, dst)\n")
	b.WriteString("  R  Replace     — hapus yang lama, download ulang\n")
	b.WriteString("  B  Block       — batalkan\n")
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  a: Add  |  r: Replace  |  b / esc: Block"))

	return b.String()
}

func (m *model) scheduleView() string {
	var b strings.Builder
	b.WriteString(titleStyle.Render(" Schedule Download "))
	b.WriteString("\n\n")
	b.WriteString("Set start time (HH:MM):\n")
	b.WriteString(m.scheduleInput.View())
	b.WriteString("\n\n")
	b.WriteString(helpStyle.Render("  enter: Confirm  |  esc: Cancel"))

	return b.String()
}

func (m *model) conflictView() string {
	if m.conflict == nil {
		m.currentPage = pageList
		return ""
	}

	var b strings.Builder
	b.WriteString(titleStyle.Render(" File Already Exists "))
	b.WriteString("\n\n")
	b.WriteString(fmt.Sprintf("File %q sudah ada di folder download.\n\n", m.conflict.filename))
	b.WriteString("Pilih aksi:\n\n")
	b.WriteString("  O  Overwrite  — timpa file lama\n")
	b.WriteString("  R  Rename    — simpan sebagai \"file (1).zip\", dst\n")
	b.WriteString("  C  Cancel    — buang hasil download\n")
	b.WriteString("\n")
	b.WriteString(helpStyle.Render("  o: Overwrite  |  r: Rename  |  c / esc: Cancel"))

	return b.String()
}
