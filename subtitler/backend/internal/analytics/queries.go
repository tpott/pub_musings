package analytics

import (
	"context"
	"fmt"
	"time"
)

// GetFunnel returns conversion funnel data for a date range
func (s *Service) GetFunnel(ctx context.Context, start, end time.Time) (*FunnelReport, error) {
	report := &FunnelReport{
		ConversionRates: make(map[string]float64),
	}
	report.Period.Start = start.Format("2006-01-02")
	report.Period.End = end.Format("2006-01-02")

	// Count distinct visitors
	var visitors int64
	err := s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT visitor_id)
		FROM analytics_events
		WHERE created_at >= ? AND created_at < ?
	`, start, end).Scan(&visitors)
	if err != nil {
		return nil, fmt.Errorf("count visitors: %w", err)
	}

	// Count distinct signups
	var signups int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT visitor_id)
		FROM analytics_events
		WHERE event_name = 'signup_completed'
		AND created_at >= ? AND created_at < ?
	`, start, end).Scan(&signups)
	if err != nil {
		return nil, fmt.Errorf("count signups: %w", err)
	}

	// Count distinct uploads
	var uploads int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT visitor_id)
		FROM analytics_events
		WHERE event_name = 'upload_completed'
		AND created_at >= ? AND created_at < ?
	`, start, end).Scan(&uploads)
	if err != nil {
		return nil, fmt.Errorf("count uploads: %w", err)
	}

	// Count distinct downloads
	var downloads int64
	err = s.db.QueryRowContext(ctx, `
		SELECT COUNT(DISTINCT visitor_id)
		FROM analytics_events
		WHERE event_name = 'download_completed'
		AND created_at >= ? AND created_at < ?
	`, start, end).Scan(&downloads)
	if err != nil {
		return nil, fmt.Errorf("count downloads: %w", err)
	}

	// Build funnel stages
	report.Funnel = []FunnelStage{
		{Stage: "visitors", Count: visitors},
		{Stage: "signups", Count: signups},
		{Stage: "uploads", Count: uploads},
		{Stage: "downloads", Count: downloads},
	}

	// Calculate conversion rates
	if visitors > 0 {
		report.ConversionRates["visitor_to_signup"] = float64(signups) / float64(visitors)
	}
	if signups > 0 {
		report.ConversionRates["signup_to_upload"] = float64(uploads) / float64(signups)
	}
	if uploads > 0 {
		report.ConversionRates["upload_to_download"] = float64(downloads) / float64(uploads)
	}

	return report, nil
}

// GetExperimentResults returns A/B test results for a specific experiment
func (s *Service) GetExperimentResults(ctx context.Context, experimentID string, start, end time.Time) (*ExperimentReport, error) {
	report := &ExperimentReport{
		ExperimentID: experimentID,
	}
	report.Period.Start = start.Format("2006-01-02")
	report.Period.End = end.Format("2006-01-02")

	// Get all variants for this experiment
	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT variant FROM analytics_experiments WHERE experiment_id = ?
	`, experimentID)
	if err != nil {
		return nil, fmt.Errorf("query variants: %w", err)
	}
	defer rows.Close()

	var variants []string
	for rows.Next() {
		var variant string
		if err := rows.Scan(&variant); err != nil {
			return nil, fmt.Errorf("scan variant: %w", err)
		}
		variants = append(variants, variant)
	}

	// For each variant, count visitors and conversions
	for _, variant := range variants {
		// Count visitors assigned to this variant
		var visitors int64
		err := s.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT visitor_id)
			FROM analytics_experiments
			WHERE experiment_id = ? AND variant = ?
		`, experimentID, variant).Scan(&visitors)
		if err != nil {
			return nil, fmt.Errorf("count visitors for variant %s: %w", variant, err)
		}

		// Count conversions (experiment_converted events) for this variant
		var conversions int64
		err = s.db.QueryRowContext(ctx, `
			SELECT COUNT(DISTINCT e.visitor_id)
			FROM analytics_events e
			JOIN analytics_experiments ex ON e.visitor_id = ex.visitor_id
			WHERE ex.experiment_id = ? AND ex.variant = ?
			AND e.event_name = 'experiment_converted'
			AND e.properties LIKE ?
			AND e.created_at >= ? AND e.created_at < ?
		`, experimentID, variant, fmt.Sprintf("%%\"experiment_id\":\"%s\"%%", experimentID), start, end).Scan(&conversions)
		if err != nil {
			return nil, fmt.Errorf("count conversions for variant %s: %w", variant, err)
		}

		// Calculate conversion rate
		conversionRate := 0.0
		if visitors > 0 {
			conversionRate = float64(conversions) / float64(visitors)
		}

		report.Variants = append(report.Variants, ExperimentVariant{
			Variant:        variant,
			Visitors:       visitors,
			Conversions:    conversions,
			ConversionRate: conversionRate,
		})
	}

	// Generate recommendation (simple heuristic for now)
	if len(report.Variants) > 1 {
		// Find control (variant A) and best variant
		var controlRate float64
		var bestVariant string
		var bestRate float64

		for _, v := range report.Variants {
			if v.Variant == "A" {
				controlRate = v.ConversionRate
			}
			if v.ConversionRate > bestRate {
				bestRate = v.ConversionRate
				bestVariant = v.Variant
			}
		}

		if bestVariant != "A" && bestRate > controlRate {
			improvement := ((bestRate - controlRate) / controlRate) * 100
			report.Recommendation = fmt.Sprintf("Variant %s shows %.1f%% improvement over control", bestVariant, improvement)
		} else {
			report.Recommendation = "No significant improvement over control"
		}
	}

	return report, nil
}
