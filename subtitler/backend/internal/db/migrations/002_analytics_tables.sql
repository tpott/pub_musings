-- Analytics tables for tracking events and experiments

-- Visitors table: tracks anonymous visitors and their UTM parameters
CREATE TABLE analytics_visitors (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    visitor_id TEXT NOT NULL UNIQUE,
    user_id INTEGER,
    utm_source TEXT,
    utm_medium TEXT,
    utm_campaign TEXT,
    first_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    last_seen_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_analytics_visitors_visitor_id ON analytics_visitors(visitor_id);
CREATE INDEX idx_analytics_visitors_user_id ON analytics_visitors(user_id);

-- Events table: stores all tracked events
CREATE TABLE analytics_events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    visitor_id TEXT NOT NULL,
    user_id INTEGER,
    event_name TEXT NOT NULL,
    properties TEXT,  -- JSON blob
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY (user_id) REFERENCES users(id) ON DELETE SET NULL
);

CREATE INDEX idx_analytics_events_visitor_id ON analytics_events(visitor_id);
CREATE INDEX idx_analytics_events_user_id ON analytics_events(user_id);
CREATE INDEX idx_analytics_events_event_name ON analytics_events(event_name);
CREATE INDEX idx_analytics_events_created_at ON analytics_events(created_at);

-- Experiments table: tracks A/B test variant assignments
CREATE TABLE analytics_experiments (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    visitor_id TEXT NOT NULL,
    experiment_id TEXT NOT NULL,
    variant TEXT NOT NULL,
    assigned_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(visitor_id, experiment_id)
);

CREATE INDEX idx_analytics_experiments_visitor_id ON analytics_experiments(visitor_id);
CREATE INDEX idx_analytics_experiments_experiment_id ON analytics_experiments(experiment_id);
