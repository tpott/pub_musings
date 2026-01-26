package db

import "embed"

// MigrationsFS embeds all migration files from the migrations directory
//
//go:embed migrations/*.sql
var MigrationsFS embed.FS

// MigrationsDir is the directory path within MigrationsFS
const MigrationsDir = "migrations"
