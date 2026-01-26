package analytics

import "time"

// Visitor represents an anonymous visitor or logged-in user
type Visitor struct {
	ID          int64     `json:"id"`
	VisitorID   string    `json:"visitor_id"`
	UserID      *int64    `json:"user_id,omitempty"`
	UTMSource   *string   `json:"utm_source,omitempty"`
	UTMMedium   *string   `json:"utm_medium,omitempty"`
	UTMCampaign *string   `json:"utm_campaign,omitempty"`
	FirstSeenAt time.Time `json:"first_seen_at"`
	LastSeenAt  time.Time `json:"last_seen_at"`
}

// Event represents a tracked analytics event
type Event struct {
	ID         int64                  `json:"id"`
	VisitorID  string                 `json:"visitor_id"`
	UserID     *int64                 `json:"user_id,omitempty"`
	EventName  string                 `json:"event_name"`
	Properties map[string]interface{} `json:"properties,omitempty"`
	CreatedAt  time.Time              `json:"created_at"`
}

// Experiment represents an A/B test variant assignment
type Experiment struct {
	ID           int64     `json:"id"`
	VisitorID    string    `json:"visitor_id"`
	ExperimentID string    `json:"experiment_id"`
	Variant      string    `json:"variant"`
	AssignedAt   time.Time `json:"assigned_at"`
}

// FunnelStage represents a single stage in a conversion funnel
type FunnelStage struct {
	Stage string `json:"stage"`
	Count int64  `json:"count"`
}

// FunnelReport represents the full funnel analysis
type FunnelReport struct {
	Period struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"period"`
	Funnel          []FunnelStage      `json:"funnel"`
	ConversionRates map[string]float64 `json:"conversion_rates"`
}

// ExperimentVariant represents results for a single variant
type ExperimentVariant struct {
	Variant        string  `json:"variant"`
	Visitors       int64   `json:"visitors"`
	Conversions    int64   `json:"conversions"`
	ConversionRate float64 `json:"conversion_rate"`
}

// ExperimentReport represents A/B test results
type ExperimentReport struct {
	ExperimentID string `json:"experiment_id"`
	Period       struct {
		Start string `json:"start"`
		End   string `json:"end"`
	} `json:"period"`
	Variants       []ExperimentVariant `json:"variants"`
	Recommendation string              `json:"recommendation,omitempty"`
}
