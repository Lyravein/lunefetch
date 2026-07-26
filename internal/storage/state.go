package storage

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"
)

type DownloadRecord struct {
	ID             int64
	URL            string
	Filename       string
	TotalSize      int64
	DownloadedSize int64
	Status         string
	SupportsRanges bool
	NumChunks      int
	QueuePosition  sql.NullInt64  // NULL = not in queue
	ScheduledAt    sql.NullString // "HH:MM", NULL = not scheduled
	CreatedAt      time.Time
	UpdatedAt      time.Time
	DeletedAt      sql.NullTime // NULL = tidak dihapus; non-NULL = masuk history
}

type ChunkRecord struct {
	ID             int64
	DownloadID     int64
	ChunkIndex     int
	StartByte      int64
	EndByte        int64
	DownloadedSize int64
	Status         string
	Error          sql.NullString
	RetryCount     int
}

type StateManager struct {
	db *sql.DB
}

func NewStateManager(dbPath string) (*StateManager, error) {
	if err := os.MkdirAll(filepath.Dir(dbPath), 0755); err != nil {
		return nil, fmt.Errorf("create db dir: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	db.SetMaxOpenConns(1)

	sm := &StateManager{db: db}
	if err := sm.migrate(); err != nil {
		return nil, fmt.Errorf("migrate: %w", err)
	}

	return sm, nil
}

func (sm *StateManager) migrate() error {
	// Additive migrations must run FIRST so that existing DBs have the new
	// columns before we try to create indexes on them.
	additive := []string{
		`ALTER TABLE downloads ADD COLUMN queue_position INTEGER`,
		`ALTER TABLE downloads ADD COLUMN scheduled_at TEXT`,
		`ALTER TABLE downloads ADD COLUMN deleted_at DATETIME`,
	}
	for _, m := range additive {
		sm.db.Exec(m) //nolint:errcheck — "duplicate column" error is expected on fresh DBs
	}

	schema := `
	CREATE TABLE IF NOT EXISTS downloads (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		url TEXT NOT NULL,
		filename TEXT NOT NULL,
		total_size INTEGER NOT NULL DEFAULT 0,
		downloaded_size INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		supports_ranges BOOLEAN NOT NULL DEFAULT 0,
		num_chunks INTEGER NOT NULL DEFAULT 1,
		queue_position INTEGER,
		scheduled_at TEXT,
		deleted_at DATETIME,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS chunks (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		download_id INTEGER NOT NULL,
		chunk_index INTEGER NOT NULL,
		start_byte INTEGER NOT NULL,
		end_byte INTEGER NOT NULL,
		downloaded_size INTEGER NOT NULL DEFAULT 0,
		status TEXT NOT NULL DEFAULT 'pending',
		error TEXT,
		retry_count INTEGER NOT NULL DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		FOREIGN KEY (download_id) REFERENCES downloads(id) ON DELETE CASCADE
	);

	CREATE INDEX IF NOT EXISTS idx_chunks_download_id ON chunks(download_id);
	CREATE INDEX IF NOT EXISTS idx_downloads_status ON downloads(status);
	CREATE INDEX IF NOT EXISTS idx_downloads_queue_position ON downloads(queue_position);
	CREATE INDEX IF NOT EXISTS idx_downloads_deleted_at ON downloads(deleted_at);
	`

	_, err := sm.db.Exec(schema)
	return err
}

func (sm *StateManager) CreateDownload(url, filename string, totalSize int64, supportsRanges bool, numChunks int) (int64, error) {
	result, err := sm.db.Exec(
		`INSERT INTO downloads (url, filename, total_size, supports_ranges, num_chunks, status)
		 VALUES (?, ?, ?, ?, ?, 'pending')`,
		url, filename, totalSize, supportsRanges, numChunks,
	)
	if err != nil {
		return 0, fmt.Errorf("create download: %w", err)
	}

	return result.LastInsertId()
}

// FindByURL returns the first non-deleted download record with the given URL,
// or nil if not found. Download yang sudah dihapus (deleted_at IS NOT NULL)
// diabaikan supaya re-download URL yang sama tidak diblokir sebagai duplikat.
func (sm *StateManager) FindByURL(url string) (*DownloadRecord, error) {
	row := sm.db.QueryRow(
		`SELECT id, url, filename, total_size, downloaded_size, status, supports_ranges, num_chunks,
		        queue_position, scheduled_at, created_at, updated_at, deleted_at
		 FROM downloads WHERE url = ? AND deleted_at IS NULL LIMIT 1`, url,
	)
	return scanDownload(row)
}

// FilenameExists returns true if a non-deleted record with the given filename
// already exists in DB. Entri yang sudah dihapus tidak dihitung supaya suffix
// generator (#1, #2) tidak tersangkut pada nama yang sudah tidak aktif.
func (sm *StateManager) FilenameExists(filename string) bool {
	var count int
	sm.db.QueryRow(
		`SELECT COUNT(*) FROM downloads WHERE filename = ? AND deleted_at IS NULL`, filename,
	).Scan(&count) //nolint:errcheck
	return count > 0
}

func (sm *StateManager) CreateChunks(downloadID int64, startBytes, endBytes []int64) error {
	tx, err := sm.db.Begin()
	if err != nil {
		return fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	stmt, err := tx.Prepare(
		`INSERT INTO chunks (download_id, chunk_index, start_byte, end_byte, status)
		 VALUES (?, ?, ?, ?, 'pending')`,
	)
	if err != nil {
		return fmt.Errorf("prepare stmt: %w", err)
	}
	defer stmt.Close()

	for i := range startBytes {
		_, err := stmt.Exec(downloadID, i, startBytes[i], endBytes[i])
		if err != nil {
			return fmt.Errorf("insert chunk %d: %w", i, err)
		}
	}

	return tx.Commit()
}

func (sm *StateManager) UpdateChunkProgress(downloadID int64, chunkIndex int, downloadedSize int64, status string) error {
	_, err := sm.db.Exec(
		`UPDATE chunks SET downloaded_size = ?, status = ?, updated_at = CURRENT_TIMESTAMP
		 WHERE download_id = ? AND chunk_index = ?`,
		downloadedSize, status, downloadID, chunkIndex,
	)
	if err != nil {
		return fmt.Errorf("update chunk: %w", err)
	}

	_, err = sm.db.Exec(
		`UPDATE downloads SET downloaded_size = (
			SELECT COALESCE(SUM(downloaded_size), 0) FROM chunks WHERE download_id = ?
		), updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		downloadID, downloadID,
	)
	return err
}

func (sm *StateManager) UpdateDownloadStatus(id int64, status string) error {
	_, err := sm.db.Exec(
		`UPDATE downloads SET status = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		status, id,
	)
	return err
}

// SetQueuePosition sets or clears the queue position of a download.
// Pass nil to remove it from the queue.
func (sm *StateManager) SetQueuePosition(id int64, pos *int64) error {
	_, err := sm.db.Exec(
		`UPDATE downloads SET queue_position = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		pos, id,
	)
	return err
}

// SetScheduledAt sets or clears the scheduled time ("HH:MM") for a download.
// Pass nil to remove the schedule.
func (sm *StateManager) SetScheduledAt(id int64, at *string) error {
	_, err := sm.db.Exec(
		`UPDATE downloads SET scheduled_at = ?, updated_at = CURRENT_TIMESTAMP WHERE id = ?`,
		at, id,
	)
	return err
}

// NextInQueue returns the download with the lowest queue_position that has
// status "queued" and is not deleted, or nil if the queue is empty.
func (sm *StateManager) NextInQueue() (*DownloadRecord, error) {
	row := sm.db.QueryRow(
		`SELECT id, url, filename, total_size, downloaded_size, status, supports_ranges, num_chunks,
		        queue_position, scheduled_at, created_at, updated_at, deleted_at
		 FROM downloads
		 WHERE status = 'queued' AND deleted_at IS NULL
		 ORDER BY queue_position ASC
		 LIMIT 1`,
	)
	rec := &DownloadRecord{}
	err := row.Scan(
		&rec.ID, &rec.URL, &rec.Filename, &rec.TotalSize,
		&rec.DownloadedSize, &rec.Status, &rec.SupportsRanges,
		&rec.NumChunks, &rec.QueuePosition, &rec.ScheduledAt,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("next in queue: %w", err)
	}
	return rec, nil
}

// ShiftQueuePositions compacts queue_position values so there are no gaps.
func (sm *StateManager) ShiftQueuePositions() error {
	rows, err := sm.db.Query(
		`SELECT id FROM downloads WHERE status = 'queued' AND deleted_at IS NULL ORDER BY queue_position ASC`,
	)
	if err != nil {
		return err
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}
		ids = append(ids, id)
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return err
	}

	tx, err := sm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	for i, id := range ids {
		pos := int64(i + 1)
		if _, err := tx.Exec(
			`UPDATE downloads SET queue_position = ? WHERE id = ?`, pos, id,
		); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// MoveQueuePosition moves a download up (-1) or down (+1) in the queue.
func (sm *StateManager) MoveQueuePosition(id int64, delta int) error {
	if err := sm.ShiftQueuePositions(); err != nil {
		return err
	}

	row := sm.db.QueryRow(`SELECT queue_position FROM downloads WHERE id = ?`, id)
	var pos sql.NullInt64
	if err := row.Scan(&pos); err != nil || !pos.Valid {
		return nil // not in queue, nothing to do
	}

	newPos := pos.Int64 + int64(delta)
	if newPos < 1 {
		newPos = 1
	}

	// Swap with whoever is at newPos.
	tx, err := sm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if _, err := tx.Exec(
		`UPDATE downloads SET queue_position = queue_position - ? WHERE queue_position = ? AND id != ?`,
		delta, newPos, id,
	); err != nil {
		return err
	}
	if _, err := tx.Exec(
		`UPDATE downloads SET queue_position = ? WHERE id = ?`, newPos, id,
	); err != nil {
		return err
	}
	return tx.Commit()
}

// ListScheduledReady returns non-deleted downloads with scheduled_at = HH:MM
// (current time) that are still in 'scheduled' status.
func (sm *StateManager) ListScheduledReady(hhmm string) ([]DownloadRecord, error) {
	rows, err := sm.db.Query(
		`SELECT id, url, filename, total_size, downloaded_size, status, supports_ranges, num_chunks,
		        queue_position, scheduled_at, created_at, updated_at, deleted_at
		 FROM downloads
		 WHERE status = 'scheduled' AND scheduled_at = ? AND deleted_at IS NULL`,
		hhmm,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var result []DownloadRecord
	for rows.Next() {
		var d DownloadRecord
		if err := rows.Scan(
			&d.ID, &d.URL, &d.Filename, &d.TotalSize, &d.DownloadedSize,
			&d.Status, &d.SupportsRanges, &d.NumChunks,
			&d.QueuePosition, &d.ScheduledAt, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt,
		); err != nil {
			return nil, err
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

func scanDownload(row *sql.Row) (*DownloadRecord, error) {
	rec := &DownloadRecord{}
	err := row.Scan(
		&rec.ID, &rec.URL, &rec.Filename, &rec.TotalSize,
		&rec.DownloadedSize, &rec.Status, &rec.SupportsRanges,
		&rec.NumChunks, &rec.QueuePosition, &rec.ScheduledAt,
		&rec.CreatedAt, &rec.UpdatedAt, &rec.DeletedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("scan download: %w", err)
	}
	return rec, nil
}

// GetDownload mengambil record berdasarkan ID tanpa filter deleted_at,
// supaya history page bisa membaca entri yang sudah dihapus.
func (sm *StateManager) GetDownload(id int64) (*DownloadRecord, error) {
	row := sm.db.QueryRow(
		`SELECT id, url, filename, total_size, downloaded_size, status, supports_ranges, num_chunks,
		        queue_position, scheduled_at, created_at, updated_at, deleted_at
		 FROM downloads WHERE id = ?`, id,
	)
	return scanDownload(row)
}

func (sm *StateManager) GetChunks(downloadID int64) ([]ChunkRecord, error) {
	rows, err := sm.db.Query(
		`SELECT id, download_id, chunk_index, start_byte, end_byte, downloaded_size, status, error, retry_count
		 FROM chunks WHERE download_id = ? ORDER BY chunk_index`, downloadID,
	)
	if err != nil {
		return nil, fmt.Errorf("query chunks: %w", err)
	}
	defer rows.Close()

	var chunks []ChunkRecord
	for rows.Next() {
		var c ChunkRecord
		if err := rows.Scan(&c.ID, &c.DownloadID, &c.ChunkIndex, &c.StartByte, &c.EndByte,
			&c.DownloadedSize, &c.Status, &c.Error, &c.RetryCount); err != nil {
			return nil, fmt.Errorf("scan chunk: %w", err)
		}
		chunks = append(chunks, c)
	}

	return chunks, rows.Err()
}

// ListDownloads mengembalikan semua download yang belum dihapus (deleted_at IS NULL).
func (sm *StateManager) ListDownloads() ([]DownloadRecord, error) {
	rows, err := sm.db.Query(
		`SELECT id, url, filename, total_size, downloaded_size, status, supports_ranges, num_chunks,
		        queue_position, scheduled_at, created_at, updated_at, deleted_at
		 FROM downloads
		 WHERE deleted_at IS NULL
		 ORDER BY
		   CASE WHEN status = 'queued' THEN 1 ELSE 0 END ASC,
		   CASE WHEN status = 'queued' THEN queue_position ELSE NULL END DESC NULLS LAST,
		   created_at ASC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list downloads: %w", err)
	}
	defer rows.Close()

	var downloads []DownloadRecord
	for rows.Next() {
		var d DownloadRecord
		if err := rows.Scan(
			&d.ID, &d.URL, &d.Filename, &d.TotalSize, &d.DownloadedSize,
			&d.Status, &d.SupportsRanges, &d.NumChunks,
			&d.QueuePosition, &d.ScheduledAt, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan download: %w", err)
		}
		downloads = append(downloads, d)
	}

	return downloads, rows.Err()
}

// DeleteDownload melakukan soft delete: set deleted_at = now.
// Status tidak diubah supaya history masih menunjukkan kondisi terakhir.
// Chunk dibiarkan utuh supaya "restore cerdas" bisa resume kalau .tmp masih ada.
// Untuk hapus file dan chunk sekaligus, gunakan DeleteWithFile.
func (sm *StateManager) DeleteDownload(id int64) error {
	_, err := sm.db.Exec(
		`UPDATE downloads SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND deleted_at IS NULL`,
		id,
	)
	return err
}

// DeleteWithFile melakukan soft delete sekaligus membersihkan chunk progress.
// Dipanggil saat user menekan D (hapus entri + file); file di-remove oleh pemanggil.
// Chunk di-reset supaya entri yang di-restore tidak mencoba resume dari progress
// yang sudah tidak valid (file-nya sudah hilang).
func (sm *StateManager) DeleteWithFile(id int64) error {
	tx, err := sm.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Soft delete entri.
	if _, err := tx.Exec(
		`UPDATE downloads SET deleted_at = CURRENT_TIMESTAMP, updated_at = CURRENT_TIMESTAMP,
		 downloaded_size = 0
		 WHERE id = ? AND deleted_at IS NULL`,
		id,
	); err != nil {
		return err
	}

	// Reset chunk progress — file sudah tidak ada, progress lama tidak valid.
	if _, err := tx.Exec(
		`UPDATE chunks SET downloaded_size = 0, status = 'pending', updated_at = CURRENT_TIMESTAMP
		 WHERE download_id = ?`,
		id,
	); err != nil {
		return err
	}

	return tx.Commit()
}

// ListDeleted mengembalikan semua download yang sudah di-soft-delete,
// diurutkan dari yang terbaru dihapus. Dipakai oleh pageHistory.
func (sm *StateManager) ListDeleted() ([]DownloadRecord, error) {
	rows, err := sm.db.Query(
		`SELECT id, url, filename, total_size, downloaded_size, status, supports_ranges, num_chunks,
		        queue_position, scheduled_at, created_at, updated_at, deleted_at
		 FROM downloads
		 WHERE deleted_at IS NOT NULL
		 ORDER BY deleted_at DESC`,
	)
	if err != nil {
		return nil, fmt.Errorf("list deleted: %w", err)
	}
	defer rows.Close()

	var result []DownloadRecord
	for rows.Next() {
		var d DownloadRecord
		if err := rows.Scan(
			&d.ID, &d.URL, &d.Filename, &d.TotalSize, &d.DownloadedSize,
			&d.Status, &d.SupportsRanges, &d.NumChunks,
			&d.QueuePosition, &d.ScheduledAt, &d.CreatedAt, &d.UpdatedAt, &d.DeletedAt,
		); err != nil {
			return nil, fmt.Errorf("scan deleted: %w", err)
		}
		result = append(result, d)
	}
	return result, rows.Err()
}

// RestoreDownload membatalkan soft delete: cleared deleted_at dan set status
// ke "paused" supaya user bisa resume dari list utama.
// Queue position dan scheduled_at dibersihkan karena tidak relevan lagi.
func (sm *StateManager) RestoreDownload(id int64) error {
	_, err := sm.db.Exec(
		`UPDATE downloads SET
		   deleted_at = NULL,
		   status = 'paused',
		   queue_position = NULL,
		   scheduled_at = NULL,
		   updated_at = CURRENT_TIMESTAMP
		 WHERE id = ? AND deleted_at IS NOT NULL`,
		id,
	)
	return err
}

// PurgeDownload menghapus satu entri history secara permanen beserta chunk-nya.
// Hanya boleh dipanggil untuk download yang sudah di-soft-delete.
func (sm *StateManager) PurgeDownload(id int64) error {
	_, err := sm.db.Exec(`DELETE FROM downloads WHERE id = ? AND deleted_at IS NOT NULL`, id)
	return err
}

// PurgeAllDeleted menghapus seluruh history secara permanen.
func (sm *StateManager) PurgeAllDeleted() error {
	_, err := sm.db.Exec(`DELETE FROM downloads WHERE deleted_at IS NOT NULL`)
	return err
}

// PurgeOlderThan menghapus entri history yang deleted_at-nya lebih tua dari
// cutoff. Dipakai untuk auto-purge retensi (misalnya 30 hari).
func (sm *StateManager) PurgeOlderThan(cutoff time.Time) error {
	_, err := sm.db.Exec(
		`DELETE FROM downloads WHERE deleted_at IS NOT NULL AND deleted_at < ?`,
		cutoff.UTC().Format("2006-01-02 15:04:05"),
	)
	return err
}

// DB exposes the raw *sql.DB for packages that need direct queries (e.g. queue).
func (sm *StateManager) DB() *sql.DB {
	return sm.db
}

func (sm *StateManager) Close() error {
	return sm.db.Close()
}
