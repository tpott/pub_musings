package db

import (
	"database/sql"
	"fmt"
	"time"
)

// CreateUploadSession creates a new chunked upload session
func (db *DB) CreateUploadSession(session *UploadSession) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO upload_sessions (id, filename, content_type, total_size, chunk_size, total_chunks, user_id, session_id, status, created_at, expires_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, session.ID, session.Filename, session.ContentType, session.TotalSize, session.ChunkSize, session.TotalChunks, session.UserID, session.SessionID, session.Status, session.CreatedAt, session.ExpiresAt)
	return err
}

// GetUploadSession retrieves an upload session by ID
func (db *DB) GetUploadSession(id string) (*UploadSession, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	session := &UploadSession{}
	var userID, sessionID sql.NullString
	var completedAt sql.NullTime

	err := db.conn.QueryRowContext(ctx, `
		SELECT id, filename, content_type, total_size, chunk_size, total_chunks, user_id, session_id, status, created_at, expires_at, completed_at
		FROM upload_sessions WHERE id = ?
	`, id).Scan(&session.ID, &session.Filename, &session.ContentType, &session.TotalSize, &session.ChunkSize, &session.TotalChunks, &userID, &sessionID, &session.Status, &session.CreatedAt, &session.ExpiresAt, &completedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	if userID.Valid {
		session.UserID = &userID.String
	}
	if sessionID.Valid {
		session.SessionID = &sessionID.String
	}
	if completedAt.Valid {
		session.CompletedAt = &completedAt.Time
	}

	return session, nil
}

// UpdateUploadSessionStatus updates the status of an upload session
func (db *DB) UpdateUploadSessionStatus(sessionID, status string) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	var completedAt interface{}
	if status == "complete" {
		completedAt = time.Now()
	}
	_, err := db.conn.ExecContext(ctx, `
		UPDATE upload_sessions SET status = ?, completed_at = ? WHERE id = ?
	`, status, completedAt, sessionID)
	return err
}

// CreateUploadChunk records a chunk upload
func (db *DB) CreateUploadChunk(chunk *UploadChunk) error {
	ctx, cancel := db.queryContext()
	defer cancel()

	_, err := db.conn.ExecContext(ctx, `
		INSERT INTO upload_chunks (id, upload_session_id, chunk_index, chunk_path, size, created_at)
		VALUES (?, ?, ?, ?, ?, ?)
	`, chunk.ID, chunk.UploadSessionID, chunk.ChunkIndex, chunk.ChunkPath, chunk.Size, chunk.CreatedAt)
	return err
}

// GetUploadChunk retrieves a specific chunk by session ID and index
func (db *DB) GetUploadChunk(sessionID string, chunkIndex int) (*UploadChunk, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	chunk := &UploadChunk{}
	err := db.conn.QueryRowContext(ctx, `
		SELECT id, upload_session_id, chunk_index, chunk_path, size, created_at
		FROM upload_chunks WHERE upload_session_id = ? AND chunk_index = ?
	`, sessionID, chunkIndex).Scan(&chunk.ID, &chunk.UploadSessionID, &chunk.ChunkIndex, &chunk.ChunkPath, &chunk.Size, &chunk.CreatedAt)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return chunk, nil
}

// GetUploadChunks retrieves all chunks for an upload session
func (db *DB) GetUploadChunks(sessionID string) ([]UploadChunk, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, upload_session_id, chunk_index, chunk_path, size, created_at
		FROM upload_chunks WHERE upload_session_id = ? ORDER BY chunk_index
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var chunks []UploadChunk
	for rows.Next() {
		var chunk UploadChunk
		if err := rows.Scan(&chunk.ID, &chunk.UploadSessionID, &chunk.ChunkIndex, &chunk.ChunkPath, &chunk.Size, &chunk.CreatedAt); err != nil {
			return nil, err
		}
		chunks = append(chunks, chunk)
	}
	return chunks, rows.Err()
}

// CountUploadChunks returns the number of chunks uploaded for a session
func (db *DB) CountUploadChunks(sessionID string) (int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `
		SELECT COUNT(*) FROM upload_chunks WHERE upload_session_id = ?
	`, sessionID).Scan(&count)
	return count, err
}

// GetReceivedChunkIndices returns the indices of all chunks received for a session
func (db *DB) GetReceivedChunkIndices(sessionID string) ([]int, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT chunk_index FROM upload_chunks WHERE upload_session_id = ? ORDER BY chunk_index
	`, sessionID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var indices []int
	for rows.Next() {
		var index int
		if err := rows.Scan(&index); err != nil {
			return nil, err
		}
		indices = append(indices, index)
	}
	return indices, rows.Err()
}

// GetTotalReceivedBytes returns the sum of all chunk sizes for a session
func (db *DB) GetTotalReceivedBytes(sessionID string) (int64, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var total sql.NullInt64
	err := db.conn.QueryRowContext(ctx, `
		SELECT SUM(size) FROM upload_chunks WHERE upload_session_id = ?
	`, sessionID).Scan(&total)
	if err != nil {
		return 0, err
	}
	if !total.Valid {
		return 0, nil
	}
	return total.Int64, nil
}

// GetExpiredUploadSessions returns all expired upload sessions
func (db *DB) GetExpiredUploadSessions() ([]UploadSession, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	rows, err := db.conn.QueryContext(ctx, `
		SELECT id, filename, content_type, total_size, chunk_size, total_chunks, user_id, session_id, status, created_at, expires_at, completed_at
		FROM upload_sessions WHERE expires_at < ? AND status = 'in_progress'
	`, time.Now())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var sessions []UploadSession
	for rows.Next() {
		var session UploadSession
		var userID, sessionID sql.NullString
		var completedAt sql.NullTime
		if err := rows.Scan(&session.ID, &session.Filename, &session.ContentType, &session.TotalSize, &session.ChunkSize, &session.TotalChunks, &userID, &sessionID, &session.Status, &session.CreatedAt, &session.ExpiresAt, &completedAt); err != nil {
			return nil, err
		}
		if userID.Valid {
			session.UserID = &userID.String
		}
		if sessionID.Valid {
			session.SessionID = &sessionID.String
		}
		if completedAt.Valid {
			session.CompletedAt = &completedAt.Time
		}
		sessions = append(sessions, session)
	}
	return sessions, rows.Err()
}

// DeleteUploadSession deletes an upload session and its chunks
// Returns the chunk paths so caller can delete files
func (db *DB) DeleteUploadSession(sessionID string) ([]string, error) {
	// Get chunk paths before deleting
	chunks, err := db.GetUploadChunks(sessionID)
	if err != nil {
		return nil, err
	}

	var paths []string
	for _, chunk := range chunks {
		paths = append(paths, chunk.ChunkPath)
	}

	// Delete chunks and session atomically
	err = db.WithTransaction(func(tx *Tx) error {
		// Delete chunks first (foreign key constraint)
		_, err := tx.tx.Exec(`DELETE FROM upload_chunks WHERE upload_session_id = ?`, sessionID)
		if err != nil {
			return fmt.Errorf("failed to delete upload chunks: %w", err)
		}

		// Delete session
		_, err = tx.tx.Exec(`DELETE FROM upload_sessions WHERE id = ?`, sessionID)
		if err != nil {
			return fmt.Errorf("failed to delete upload session: %w", err)
		}

		return nil
	})
	if err != nil {
		return nil, err
	}

	return paths, nil
}

// UploadSessionExists checks if an upload session exists in the database
func (db *DB) UploadSessionExists(sessionID string) (bool, error) {
	ctx, cancel := db.queryContext()
	defer cancel()

	var count int
	err := db.conn.QueryRowContext(ctx, `SELECT COUNT(*) FROM upload_sessions WHERE id = ?`, sessionID).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}
