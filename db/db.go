// Package db provides a SQLite-backed persistence layer for the deploy tool.
//
// It stores deployment execution records (status, output, timestamps) and
// user-defined key-value configuration. The database is created automatically
// on first use and migrations are applied on Open.
package db

import (
	"database/sql"
	"fmt"
	"time"

	_ "modernc.org/sqlite"

	"github.com/smavropoulos/deploy/types"
)

// DB wraps the SQLite connection.
type DB struct {
	conn *sql.DB
}

// Open opens (or creates) the SQLite database at the given path and runs migrations.
func Open(path string) (*DB, error) {
	conn, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open db: %w", err)
	}
	if err := migrate(conn); err != nil {
		conn.Close()
		return nil, err
	}
	return &DB{conn: conn}, nil
}

// Close closes the database connection.
func (d *DB) Close() error {
	return d.conn.Close()
}

func migrate(conn *sql.DB) error {
	ddl := `
	CREATE TABLE IF NOT EXISTS deployments (
		id          INTEGER PRIMARY KEY AUTOINCREMENT,
		name        TEXT NOT NULL,
		hash        TEXT NOT NULL,
		description TEXT,
		file_path   TEXT NOT NULL,
		status      TEXT NOT NULL DEFAULT 'pending',
		plugin_name TEXT NOT NULL DEFAULT 'shell',
		started_at  DATETIME,
		finished_at DATETIME,
		output      TEXT,
		created_at  DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);
	CREATE INDEX IF NOT EXISTS idx_deployments_name ON deployments(name);
	CREATE INDEX IF NOT EXISTS idx_deployments_hash ON deployments(hash);
	CREATE INDEX IF NOT EXISTS idx_deployments_status ON deployments(status);

	CREATE TABLE IF NOT EXISTS config (
		key        TEXT PRIMARY KEY,
		value      TEXT NOT NULL,
		updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS plugins (
		name         TEXT PRIMARY KEY,
		source       TEXT NOT NULL,
		version      TEXT NOT NULL DEFAULT '',
		exec_path    TEXT NOT NULL,
		installed_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
		last_used_at DATETIME
	);
	`
	_, err := conn.Exec(ddl)
	return err
}

// --- Deployment operations ---

// InsertDeployment creates a new deployment record and returns its ID.
func (d *DB) InsertDeployment(name, hash, description, filePath, pluginName string) (int64, error) {
	res, err := d.conn.Exec(
		`INSERT INTO deployments (name, hash, description, file_path, plugin_name, status, started_at)
		 VALUES (?, ?, ?, ?, ?, 'running', ?)`,
		name, hash, description, filePath, pluginName, time.Now().UTC(),
	)
	if err != nil {
		return 0, err
	}
	return res.LastInsertId()
}

// UpdateDeploymentStatus sets the final status and output of a deployment.
func (d *DB) UpdateDeploymentStatus(id int64, status, output string) error {
	_, err := d.conn.Exec(
		`UPDATE deployments SET status = ?, output = ?, finished_at = ? WHERE id = ?`,
		status, output, time.Now().UTC(), id,
	)
	return err
}

// GetDeployment returns a deployment by ID.
func (d *DB) GetDeployment(id int64) (*types.DeploymentRecord, error) {
	row := d.conn.QueryRow(
		`SELECT id, name, hash, description, file_path, status, plugin_name, started_at, finished_at, output, created_at
		 FROM deployments WHERE id = ?`, id,
	)
	return scanDeployment(row)
}

// QueryDeployments searches deployments by an optional filter (name, hash, status, or description substring).
func (d *DB) QueryDeployments(filter string) ([]types.DeploymentRecord, error) {
	query := `SELECT id, name, hash, description, file_path, status, plugin_name, started_at, finished_at, output, created_at
		FROM deployments`
	var args []any
	if filter != "" {
		query += ` WHERE name = ? OR hash = ? OR status = ? OR description LIKE ?`
		args = append(args, filter, filter, filter, "%"+filter+"%")
	}
	query += ` ORDER BY created_at DESC`

	rows, err := d.conn.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []types.DeploymentRecord
	for rows.Next() {
		var r types.DeploymentRecord
		var finishedAt sql.NullTime
		if err := rows.Scan(&r.ID, &r.Name, &r.Hash, &r.Description, &r.FilePath,
			&r.Status, &r.PluginName, &r.StartedAt, &finishedAt, &r.Output, &r.CreatedAt); err != nil {
			return nil, err
		}
		if finishedAt.Valid {
			r.FinishedAt = &finishedAt.Time
		}
		results = append(results, r)
	}
	return results, rows.Err()
}

func scanDeployment(row *sql.Row) (*types.DeploymentRecord, error) {
	var r types.DeploymentRecord
	var finishedAt sql.NullTime
	if err := row.Scan(&r.ID, &r.Name, &r.Hash, &r.Description, &r.FilePath,
		&r.Status, &r.PluginName, &r.StartedAt, &finishedAt, &r.Output, &r.CreatedAt); err != nil {
		return nil, err
	}
	if finishedAt.Valid {
		r.FinishedAt = &finishedAt.Time
	}
	return &r, nil
}

// --- Config operations ---

// SetConfig sets a key-value pair.
func (d *DB) SetConfig(key, value string) error {
	_, err := d.conn.Exec(
		`INSERT INTO config (key, value, updated_at) VALUES (?, ?, ?)
		 ON CONFLICT(key) DO UPDATE SET value = excluded.value, updated_at = excluded.updated_at`,
		key, value, time.Now().UTC(),
	)
	return err
}

// GetConfig retrieves a config value by key.
func (d *DB) GetConfig(key string) (*types.ConfigEntry, error) {
	var c types.ConfigEntry
	err := d.conn.QueryRow(`SELECT key, value, updated_at FROM config WHERE key = ?`, key).
		Scan(&c.Key, &c.Value, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// DeleteConfig removes a config key.
func (d *DB) DeleteConfig(key string) error {
	res, err := d.conn.Exec(`DELETE FROM config WHERE key = ?`, key)
	if err != nil {
		return err
	}
	n, _ := res.RowsAffected()
	if n == 0 {
		return fmt.Errorf("config key %q not found", key)
	}
	return nil
}

// ListConfig returns all config entries.
func (d *DB) ListConfig() ([]types.ConfigEntry, error) {
	rows, err := d.conn.Query(`SELECT key, value, updated_at FROM config ORDER BY key`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var entries []types.ConfigEntry
	for rows.Next() {
		var c types.ConfigEntry
		if err := rows.Scan(&c.Key, &c.Value, &c.UpdatedAt); err != nil {
			return nil, err
		}
		entries = append(entries, c)
	}
	return entries, rows.Err()
}

// --- Plugin operations ---

// UpsertPlugin records a plugin, updating it if it already exists.
func (d *DB) UpsertPlugin(name, source, version, execPath string) error {
	_, err := d.conn.Exec(
		`INSERT INTO plugins (name, source, version, exec_path, installed_at)
		 VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(name) DO UPDATE SET
		   source = excluded.source,
		   version = excluded.version,
		   exec_path = excluded.exec_path,
		   installed_at = excluded.installed_at`,
		name, source, version, execPath, time.Now().UTC(),
	)
	return err
}

// TouchPluginUsed updates the last_used_at timestamp for a plugin.
func (d *DB) TouchPluginUsed(name string) error {
	_, err := d.conn.Exec(
		`UPDATE plugins SET last_used_at = ? WHERE name = ?`,
		time.Now().UTC(), name,
	)
	return err
}

// ListPlugins returns all registered plugins.
func (d *DB) ListPlugins() ([]types.PluginRecord, error) {
	rows, err := d.conn.Query(
		`SELECT name, source, version, exec_path, installed_at, last_used_at FROM plugins ORDER BY name`,
	)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var plugins []types.PluginRecord
	for rows.Next() {
		var p types.PluginRecord
		var lastUsed sql.NullTime
		if err := rows.Scan(&p.Name, &p.Source, &p.Version, &p.ExecPath,
			&p.InstalledAt, &lastUsed); err != nil {
			return nil, err
		}
		if lastUsed.Valid {
			p.LastUsedAt = &lastUsed.Time
		}
		plugins = append(plugins, p)
	}
	return plugins, rows.Err()
}
