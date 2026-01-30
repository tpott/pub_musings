package db

import "time"

// Video represents an uploaded video
type Video struct {
	ID                    string    `json:"id"`
	Filename              string    `json:"filename"`
	Size                  int64     `json:"size"`
	ContentType           string    `json:"content_type"`
	FilePath              string    `json:"file_path"`
	ThumbnailPath         *string   `json:"thumbnail_path,omitempty"`          // path to encrypted thumbnail image
	KeyVersion            int       `json:"key_version"`                       // encryption key version (for key rotation)
	EmbeddedSubtitlesJSON *string   `json:"embedded_subtitles_json,omitempty"` // JSON-encoded embedded subtitle tracks
	CreatedAt             time.Time `json:"created_at"`
	UserID                *string   `json:"user_id,omitempty"` // null for anonymous uploads
	SessionID             *string   `json:"session_id,omitempty"`
}

// Transcription represents a transcription job and its result
type Transcription struct {
	ID           string     `json:"id"`
	VideoID      string     `json:"video_id"`
	Status       string     `json:"status"` // pending, processing, complete, error
	Message      string     `json:"message,omitempty"`
	Progress     int        `json:"progress"`
	Language     string     `json:"language,omitempty"`
	Duration     float64    `json:"duration,omitempty"`
	FullText     string     `json:"full_text,omitempty"`
	SegmentsJSON string     `json:"-"` // JSON-encoded segments
	CreatedAt    time.Time  `json:"created_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
}

// Segment represents a single subtitle segment
type Segment struct {
	ID    int     `json:"id"`
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// Role constants for user authorization
const (
	RoleUser  = "user"
	RoleAdmin = "admin"
)

// User represents a registered user
type User struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	PasswordHash  string     `json:"-"` // Never serialize
	TOTPSecret    *string    `json:"-"` // Never serialize, nil if 2FA not set up
	TOTPEnabled   bool       `json:"totp_enabled"`
	EmailVerified bool       `json:"email_verified"`
	VerifiedAt    *time.Time `json:"verified_at,omitempty"`
	Role          string     `json:"role"` // "user" or "admin"
	CreatedAt     time.Time  `json:"created_at"`
}

// IsAdmin returns true if the user has admin role
func (u *User) IsAdmin() bool {
	return u.Role == RoleAdmin
}

// Session represents an authenticated session
type Session struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	Token     string    `json:"-"` // Never serialize token in JSON
	IPAddress string    `json:"ip_address,omitempty"`
	UserAgent string    `json:"user_agent,omitempty"`
	ExpiresAt time.Time `json:"expires_at"`
	CreatedAt time.Time `json:"created_at"`
}

// BurnJob represents a subtitle burning job
type BurnJob struct {
	ID               string     `json:"id"`
	VideoID          string     `json:"video_id"`
	Status           string     `json:"status"` // pending, processing, complete, error
	Message          string     `json:"message,omitempty"`
	Progress         int        `json:"progress"`
	OutputPath       string     `json:"output_path,omitempty"`
	OutputKeyVersion *int       `json:"output_key_version,omitempty"` // encryption key version for output file
	CreatedAt        time.Time  `json:"created_at"`
	CompletedAt      *time.Time `json:"completed_at,omitempty"`
}

// RecoveryCode represents a hashed 2FA recovery code
type RecoveryCode struct {
	ID        string     `json:"id"`
	UserID    string     `json:"user_id"`
	CodeHash  string     `json:"-"` // Never serialize
	Used      bool       `json:"used"`
	CreatedAt time.Time  `json:"created_at"`
	UsedAt    *time.Time `json:"used_at,omitempty"`
}

// PasswordResetToken represents a password reset request token
type PasswordResetToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // Never serialize, SHA-256 hash of actual token
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// EmailVerificationToken represents an email verification token
type EmailVerificationToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // Never serialize, SHA-256 hash of actual token
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// MagicLinkToken represents a passwordless login token
type MagicLinkToken struct {
	ID        string    `json:"id"`
	UserID    string    `json:"user_id"`
	TokenHash string    `json:"-"` // Never serialize, SHA-256 hash of actual token
	ExpiresAt time.Time `json:"expires_at"`
	Used      bool      `json:"used"`
	CreatedAt time.Time `json:"created_at"`
}

// LoginAttempt tracks failed login attempts for rate limiting
type LoginAttempt struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Success   bool      `json:"success"`
	IPAddress string    `json:"ip_address,omitempty"`
	CreatedAt time.Time `json:"created_at"`
}

// UploadSession represents a chunked upload session
type UploadSession struct {
	ID          string     `json:"id"`
	Filename    string     `json:"filename"`
	ContentType string     `json:"content_type"`
	TotalSize   int64      `json:"total_size"`
	ChunkSize   int64      `json:"chunk_size"`
	TotalChunks int        `json:"total_chunks"`
	UserID      *string    `json:"user_id,omitempty"`
	SessionID   *string    `json:"session_id,omitempty"`
	Status      string     `json:"status"` // in_progress, complete, expired
	CreatedAt   time.Time  `json:"created_at"`
	ExpiresAt   time.Time  `json:"expires_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// UploadChunk represents a single chunk within an upload session
type UploadChunk struct {
	ID              string    `json:"id"`
	UploadSessionID string    `json:"upload_session_id"`
	ChunkIndex      int       `json:"chunk_index"`
	ChunkPath       string    `json:"chunk_path"`
	Size            int64     `json:"size"`
	CreatedAt       time.Time `json:"created_at"`
}

// Feedback status constants
const (
	FeedbackStatusNew      = "new"
	FeedbackStatusRead     = "read"
	FeedbackStatusResolved = "resolved"
)

// Feedback type constants
const (
	FeedbackTypeGeneral = "general"
	FeedbackTypeBug     = "bug"
	FeedbackTypeFeature = "feature"
)

// Feedback represents user feedback
type Feedback struct {
	ID          string    `json:"id"`
	UserID      *string   `json:"user_id,omitempty"`    // NULL for anonymous users
	SessionID   *string   `json:"session_id,omitempty"` // For anonymous tracking
	VideoID     *string   `json:"video_id,omitempty"`   // Which video (optional)
	PageURL     string    `json:"page_url"`             // Current page URL
	Text        string    `json:"text"`                 // User's message
	Rating      *int      `json:"rating,omitempty"`     // 1-5 or NULL
	Type        string    `json:"type"`                 // "general", "bug", "feature"
	BrowserInfo string    `json:"browser_info"`         // User agent, viewport, etc.
	CreatedAt   time.Time `json:"created_at"`
	Status      string    `json:"status"` // "new", "read", "resolved"
}

// VideoListResult contains paginated video results
type VideoListResult struct {
	Videos     []Video
	TotalCount int
}

// DeletedVideoFiles contains paths to files that should be deleted after a video is removed
type DeletedVideoFiles struct {
	FilePath       string  // path to the video file
	ThumbnailPath  *string // path to the thumbnail file (may be nil)
	BurnOutputPath *string // path to the burned video file (may be nil)
}
