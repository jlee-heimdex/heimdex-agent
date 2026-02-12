package catalog

import (
	"context"
	"database/sql"
	"time"
)

type Repository interface {
	CreateSource(ctx context.Context, source *Source) error
	GetSource(ctx context.Context, id string) (*Source, error)
	GetSourceByPath(ctx context.Context, path string) (*Source, error)
	ListSources(ctx context.Context) ([]*Source, error)
	DeleteSource(ctx context.Context, id string) error
	UpdateSourcePresent(ctx context.Context, id string, present bool) error
	UpdateSourceCloudLibraryID(ctx context.Context, id, cloudLibraryID string) error

	CreateFile(ctx context.Context, file *File) error
	GetFile(ctx context.Context, id string) (*File, error)
	ListFiles(ctx context.Context) ([]*File, error)
	GetFilesBySource(ctx context.Context, sourceID string) ([]*File, error)
	DeleteFilesBySource(ctx context.Context, sourceID string) error
	UpsertFile(ctx context.Context, file *File) error
	CountFiles(ctx context.Context) (int, error)

	CreateJob(ctx context.Context, job *Job) error
	GetJob(ctx context.Context, id string) (*Job, error)
	ListJobs(ctx context.Context, limit int) ([]*Job, error)
	ListPendingJobs(ctx context.Context) ([]*Job, error)
	UpdateJobStatus(ctx context.Context, id, status, errorMsg string) error
	UpdateJobProgress(ctx context.Context, id string, progress int) error

	GetConfig(ctx context.Context, key string) (string, error)
	SetConfig(ctx context.Context, key, value string) error
}

type SQLiteRepository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *SQLiteRepository {
	return &SQLiteRepository{db: db}
}

func (r *SQLiteRepository) CreateSource(ctx context.Context, s *Source) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO sources (id, type, path, display_name, drive_nickname, cloud_library_id, present, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, s.ID, s.Type, s.Path, s.DisplayName, nullString(s.DriveNickname), nullString(s.CloudLibraryID), boolToInt(s.Present), s.CreatedAt.Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) GetSource(ctx context.Context, id string) (*Source, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, type, path, display_name, drive_nickname, cloud_library_id, present, created_at
		FROM sources WHERE id = ?
	`, id)
	return r.scanSource(row)
}

func (r *SQLiteRepository) GetSourceByPath(ctx context.Context, path string) (*Source, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, type, path, display_name, drive_nickname, cloud_library_id, present, created_at
		FROM sources WHERE path = ?
	`, path)
	return r.scanSource(row)
}

func (r *SQLiteRepository) scanSource(row *sql.Row) (*Source, error) {
	var s Source
	var present int
	var createdAt string
	var driveNickname sql.NullString
	var cloudLibraryID sql.NullString

	err := row.Scan(&s.ID, &s.Type, &s.Path, &s.DisplayName, &driveNickname, &cloudLibraryID, &present, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	s.Present = present == 1
	s.DriveNickname = driveNickname.String
	s.CloudLibraryID = cloudLibraryID.String
	s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &s, nil
}

func (r *SQLiteRepository) ListSources(ctx context.Context) ([]*Source, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, type, path, display_name, drive_nickname, cloud_library_id, present, created_at
		FROM sources ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sources []*Source
	for rows.Next() {
		var s Source
		var present int
		var createdAt string
		var driveNickname sql.NullString
		var cloudLibraryID sql.NullString

		if err := rows.Scan(&s.ID, &s.Type, &s.Path, &s.DisplayName, &driveNickname, &cloudLibraryID, &present, &createdAt); err != nil {
			return nil, err
		}
		s.Present = present == 1
		s.DriveNickname = driveNickname.String
		s.CloudLibraryID = cloudLibraryID.String
		s.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		sources = append(sources, &s)
	}
	return sources, rows.Err()
}

func (r *SQLiteRepository) DeleteSource(ctx context.Context, id string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM sources WHERE id = ?", id)
	return err
}

func (r *SQLiteRepository) UpdateSourcePresent(ctx context.Context, id string, present bool) error {
	_, err := r.db.ExecContext(ctx, "UPDATE sources SET present = ? WHERE id = ?", boolToInt(present), id)
	return err
}

func (r *SQLiteRepository) UpdateSourceCloudLibraryID(ctx context.Context, id, cloudLibraryID string) error {
	_, err := r.db.ExecContext(ctx, "UPDATE sources SET cloud_library_id = ? WHERE id = ?", cloudLibraryID, id)
	return err
}

func (r *SQLiteRepository) CreateFile(ctx context.Context, f *File) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO files (id, source_id, path, filename, size, mtime, fingerprint, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
	`, f.ID, f.SourceID, f.Path, f.Filename, f.Size, f.Mtime.Format(time.RFC3339), f.Fingerprint, f.CreatedAt.Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) GetFile(ctx context.Context, id string) (*File, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, source_id, path, filename, size, mtime, fingerprint, created_at
		FROM files WHERE id = ?
	`, id)

	var f File
	var mtime, createdAt string
	err := row.Scan(&f.ID, &f.SourceID, &f.Path, &f.Filename, &f.Size, &mtime, &f.Fingerprint, &createdAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	f.Mtime, _ = time.Parse(time.RFC3339, mtime)
	f.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	return &f, nil
}

func (r *SQLiteRepository) ListFiles(ctx context.Context) ([]*File, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source_id, path, filename, size, mtime, fingerprint, created_at
		FROM files ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		var f File
		var mtime, createdAt string
		if err := rows.Scan(&f.ID, &f.SourceID, &f.Path, &f.Filename, &f.Size, &mtime, &f.Fingerprint, &createdAt); err != nil {
			return nil, err
		}
		f.Mtime, _ = time.Parse(time.RFC3339, mtime)
		f.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		files = append(files, &f)
	}
	return files, rows.Err()
}

func (r *SQLiteRepository) GetFilesBySource(ctx context.Context, sourceID string) ([]*File, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, source_id, path, filename, size, mtime, fingerprint, created_at
		FROM files WHERE source_id = ? ORDER BY filename
	`, sourceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var files []*File
	for rows.Next() {
		var f File
		var mtime, createdAt string
		if err := rows.Scan(&f.ID, &f.SourceID, &f.Path, &f.Filename, &f.Size, &mtime, &f.Fingerprint, &createdAt); err != nil {
			return nil, err
		}
		f.Mtime, _ = time.Parse(time.RFC3339, mtime)
		f.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		files = append(files, &f)
	}
	return files, rows.Err()
}

func (r *SQLiteRepository) DeleteFilesBySource(ctx context.Context, sourceID string) error {
	_, err := r.db.ExecContext(ctx, "DELETE FROM files WHERE source_id = ?", sourceID)
	return err
}

func (r *SQLiteRepository) UpsertFile(ctx context.Context, f *File) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO files (id, source_id, path, filename, size, mtime, fingerprint, created_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(source_id, path) DO UPDATE SET
			size = excluded.size,
			mtime = excluded.mtime,
			fingerprint = excluded.fingerprint
	`, f.ID, f.SourceID, f.Path, f.Filename, f.Size, f.Mtime.Format(time.RFC3339), f.Fingerprint, f.CreatedAt.Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) CountFiles(ctx context.Context) (int, error) {
	var count int
	err := r.db.QueryRowContext(ctx, "SELECT COUNT(*) FROM files").Scan(&count)
	return count, err
}

func (r *SQLiteRepository) CreateJob(ctx context.Context, j *Job) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO jobs (id, type, status, source_id, file_id, progress, error, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, j.ID, j.Type, j.Status, nullString(j.SourceID), nullString(j.FileID),
		j.Progress, nullString(j.Error),
		j.CreatedAt.Format(time.RFC3339), j.UpdatedAt.Format(time.RFC3339))
	return err
}

func (r *SQLiteRepository) GetJob(ctx context.Context, id string) (*Job, error) {
	row := r.db.QueryRowContext(ctx, `
		SELECT id, type, status, source_id, file_id, progress, error, created_at, updated_at
		FROM jobs WHERE id = ?
	`, id)
	return r.scanJob(row)
}

func (r *SQLiteRepository) scanJob(row *sql.Row) (*Job, error) {
	var j Job
	var sourceID, fileID, errMsg sql.NullString
	var createdAt, updatedAt string

	err := row.Scan(&j.ID, &j.Type, &j.Status, &sourceID, &fileID, &j.Progress, &errMsg, &createdAt, &updatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	j.SourceID = sourceID.String
	j.FileID = fileID.String
	j.Error = errMsg.String
	j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
	j.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
	return &j, nil
}

func (r *SQLiteRepository) ListJobs(ctx context.Context, limit int) ([]*Job, error) {
	if limit <= 0 {
		limit = 50
	}
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, type, status, source_id, file_id, progress, error, created_at, updated_at
		FROM jobs ORDER BY created_at DESC LIMIT ?
	`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanJobs(rows)
}

func (r *SQLiteRepository) ListPendingJobs(ctx context.Context) ([]*Job, error) {
	rows, err := r.db.QueryContext(ctx, `
		SELECT id, type, status, source_id, file_id, progress, error, created_at, updated_at
		FROM jobs WHERE status = 'pending' ORDER BY created_at ASC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return r.scanJobs(rows)
}

func (r *SQLiteRepository) scanJobs(rows *sql.Rows) ([]*Job, error) {
	var jobs []*Job
	for rows.Next() {
		var j Job
		var sourceID, fileID, errMsg sql.NullString
		var createdAt, updatedAt string

		if err := rows.Scan(&j.ID, &j.Type, &j.Status, &sourceID, &fileID, &j.Progress, &errMsg, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		j.SourceID = sourceID.String
		j.FileID = fileID.String
		j.Error = errMsg.String
		j.CreatedAt, _ = time.Parse(time.RFC3339, createdAt)
		j.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAt)
		jobs = append(jobs, &j)
	}
	return jobs, rows.Err()
}

func (r *SQLiteRepository) UpdateJobStatus(ctx context.Context, id, status, errorMsg string) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE jobs SET status = ?, error = ?, updated_at = datetime('now') WHERE id = ?
	`, status, nullString(errorMsg), id)
	return err
}

func (r *SQLiteRepository) UpdateJobProgress(ctx context.Context, id string, progress int) error {
	_, err := r.db.ExecContext(ctx, `
		UPDATE jobs SET progress = ?, updated_at = datetime('now') WHERE id = ?
	`, progress, id)
	return err
}

func (r *SQLiteRepository) GetConfig(ctx context.Context, key string) (string, error) {
	var value string
	err := r.db.QueryRowContext(ctx, "SELECT value FROM config WHERE key = ?", key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (r *SQLiteRepository) SetConfig(ctx context.Context, key, value string) error {
	_, err := r.db.ExecContext(ctx, `
		INSERT INTO config (key, value) VALUES (?, ?)
		ON CONFLICT(key) DO UPDATE SET value = excluded.value
	`, key, value)
	return err
}

func boolToInt(b bool) int {
	if b {
		return 1
	}
	return 0
}

func nullString(s string) sql.NullString {
	if s == "" {
		return sql.NullString{}
	}
	return sql.NullString{String: s, Valid: true}
}
