package analytics

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"math/rand"
)

// Service provides analytics tracking and querying
type Service struct {
	db *sql.DB
}

// NewService creates a new analytics service
func NewService(db *sql.DB) *Service {
	return &Service{db: db}
}

// TrackEvent records an analytics event
func (s *Service) TrackEvent(ctx context.Context, visitorID string, userID *int64, eventName string, properties map[string]interface{}, utmSource, utmMedium, utmCampaign *string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer tx.Rollback()

	// Upsert visitor
	if err := s.upsertVisitor(ctx, tx, visitorID, userID, utmSource, utmMedium, utmCampaign); err != nil {
		return fmt.Errorf("upsert visitor: %w", err)
	}

	// Serialize properties to JSON
	var propertiesJSON []byte
	if properties != nil {
		propertiesJSON, err = json.Marshal(properties)
		if err != nil {
			return fmt.Errorf("marshal properties: %w", err)
		}
	}

	// Insert event
	query := `
		INSERT INTO analytics_events (visitor_id, user_id, event_name, properties)
		VALUES (?, ?, ?, ?)
	`
	if _, err := tx.ExecContext(ctx, query, visitorID, userID, eventName, propertiesJSON); err != nil {
		return fmt.Errorf("insert event: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit transaction: %w", err)
	}

	return nil
}

// upsertVisitor updates visitor's last_seen_at or creates new visitor record
func (s *Service) upsertVisitor(ctx context.Context, tx *sql.Tx, visitorID string, userID *int64, utmSource, utmMedium, utmCampaign *string) error {
	// Check if visitor exists
	var exists bool
	err := tx.QueryRowContext(ctx, "SELECT EXISTS(SELECT 1 FROM analytics_visitors WHERE visitor_id = ?)", visitorID).Scan(&exists)
	if err != nil {
		return fmt.Errorf("check visitor exists: %w", err)
	}

	if exists {
		// Update last_seen_at
		query := `UPDATE analytics_visitors SET last_seen_at = CURRENT_TIMESTAMP, user_id = ? WHERE visitor_id = ?`
		if _, err := tx.ExecContext(ctx, query, userID, visitorID); err != nil {
			return fmt.Errorf("update visitor: %w", err)
		}
	} else {
		// Insert new visitor
		query := `
			INSERT INTO analytics_visitors (visitor_id, user_id, utm_source, utm_medium, utm_campaign)
			VALUES (?, ?, ?, ?, ?)
		`
		if _, err := tx.ExecContext(ctx, query, visitorID, userID, utmSource, utmMedium, utmCampaign); err != nil {
			return fmt.Errorf("insert visitor: %w", err)
		}
	}

	return nil
}

// AssignExperiment assigns a visitor to an experiment variant
func (s *Service) AssignExperiment(ctx context.Context, visitorID, experimentID string, variants []string) (string, error) {
	// Check if visitor already assigned
	var existingVariant string
	err := s.db.QueryRowContext(ctx,
		"SELECT variant FROM analytics_experiments WHERE visitor_id = ? AND experiment_id = ?",
		visitorID, experimentID,
	).Scan(&existingVariant)

	if err == nil {
		// Already assigned, return existing variant
		return existingVariant, nil
	} else if err != sql.ErrNoRows {
		return "", fmt.Errorf("query experiment assignment: %w", err)
	}

	// Not assigned yet, randomly choose a variant
	if len(variants) == 0 {
		return "", fmt.Errorf("no variants provided")
	}
	variant := variants[rand.Intn(len(variants))]

	// Insert assignment
	query := `
		INSERT INTO analytics_experiments (visitor_id, experiment_id, variant)
		VALUES (?, ?, ?)
	`
	if _, err := s.db.ExecContext(ctx, query, visitorID, experimentID, variant); err != nil {
		return "", fmt.Errorf("insert experiment assignment: %w", err)
	}

	return variant, nil
}

// GetExperimentVariant retrieves the assigned variant for a visitor
func (s *Service) GetExperimentVariant(ctx context.Context, visitorID, experimentID string) (string, error) {
	var variant string
	err := s.db.QueryRowContext(ctx,
		"SELECT variant FROM analytics_experiments WHERE visitor_id = ? AND experiment_id = ?",
		visitorID, experimentID,
	).Scan(&variant)

	if err == sql.ErrNoRows {
		return "", nil
	} else if err != nil {
		return "", fmt.Errorf("query experiment variant: %w", err)
	}

	return variant, nil
}
