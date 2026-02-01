package storage

import (
	"database/sql"
	"fmt"
	"strings"
	"sync"
	"time"

	_ "modernc.org/sqlite"
)

// safeConfigKeys 定义可以安全跨设备和跨平台备份/恢复的 app_config 配置项。
// 这些配置是平台无关的，不包含设备特定或路径相关的值。
// 不在此列表中的配置项（如 device_id、terminal_*、backup_local_dir、proxy_url 等）
// 是设备/平台特定的，不应在不同设备间同步。
var safeConfigKeys = []string{
	// 应用设置
	"port", "logLevel", "language",
	// 主题设置
	"theme", "themeAuto", "autoLightTheme", "autoDarkTheme",
	// 窗口关闭行为
	"closeWindowBehavior",
	// WebDAV 设置（URL 和凭证是通用的）
	"webdav_url", "webdav_username", "webdav_password", "webdav_configPath", "webdav_statsPath",
	// 备份提供商类型（不包括本地路径）
	"backup_provider",
	// S3 设置（云配置是通用的）
	"backup_s3_endpoint", "backup_s3_region", "backup_s3_bucket", "backup_s3_prefix",
	"backup_s3_accessKey", "backup_s3_secretKey", "backup_s3_sessionToken",
	"backup_s3_useSSL", "backup_s3_forcePathStyle",
	// 更新设置
	"update_autoCheck", "update_checkInterval",
	// 路由策略设置
	"routing_enableModelRouting", "routing_enableLoadBalance",
	"routing_enableCostPriority", "routing_enableQuotaRouting",
	"routing_loadBalanceAlgorithm",
}

type SQLiteStorage struct {
	db     *sql.DB
	dbPath string
	mu     sync.RWMutex
}

func NewSQLiteStorage(dbPath string) (*SQLiteStorage, error) {
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, err
	}

	s := &SQLiteStorage{
		db:     db,
		dbPath: dbPath,
	}
	if err := s.initSchema(); err != nil {
		db.Close()
		return nil, err
	}

	return s, nil
}

func (s *SQLiteStorage) initSchema() error {
	schema := `
	CREATE TABLE IF NOT EXISTS endpoints (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		name TEXT NOT NULL,
		api_url TEXT NOT NULL,
		api_key TEXT NOT NULL,
		enabled BOOLEAN DEFAULT TRUE,
		transformer TEXT DEFAULT 'claude',
		model TEXT,
		remark TEXT,
		sort_order INTEGER DEFAULT 0,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE TABLE IF NOT EXISTS daily_stats (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		endpoint_name TEXT NOT NULL,
		client_type TEXT DEFAULT 'claude',
		date TEXT NOT NULL,
		requests INTEGER DEFAULT 0,
		errors INTEGER DEFAULT 0,
		input_tokens INTEGER DEFAULT 0,
		output_tokens INTEGER DEFAULT 0,
		device_id TEXT DEFAULT 'default',
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
		UNIQUE(endpoint_name, client_type, date, device_id)
	);

	CREATE TABLE IF NOT EXISTS app_config (
		key TEXT PRIMARY KEY,
		value TEXT,
		updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);

	CREATE INDEX IF NOT EXISTS idx_daily_stats_date ON daily_stats(date);
	CREATE INDEX IF NOT EXISTS idx_daily_stats_endpoint ON daily_stats(endpoint_name);
	CREATE INDEX IF NOT EXISTS idx_daily_stats_device ON daily_stats(device_id);
	`

	if _, err := s.db.Exec(schema); err != nil {
		return err
	}

	// Migrations
	if err := s.migrateSortOrder(); err != nil {
		return err
	}

	if err := s.migrateCacheTokens(); err != nil {
		return err
	}

	if err := s.migrateRequestStats(); err != nil {
		return err
	}

	if err := s.migrateClientType(); err != nil {
		return err
	}

	if err := s.migrateClientIP(); err != nil {
		return err
	}

	if err := s.migrateErrorMessage(); err != nil {
		return err
	}

	if err := s.migrateEndpointTags(); err != nil {
		return err
	}

	if err := s.migrateHealthHistory(); err != nil {
		return err
	}

	if err := s.migrateRoutingFields(); err != nil {
		return err
	}

	if err := s.migrateEndpointQuotas(); err != nil {
		return err
	}

	// 迁移端点状态字段
	if err := s.migrateEndpointStatus(); err != nil {
		return err
	}

	return nil
}

// migrateSortOrder adds the sort_order column to existing databases
func (s *SQLiteStorage) migrateSortOrder() error {
	// Check if sort_order column exists
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='sort_order'`).Scan(&count)
	if err != nil {
		return err
	}

	// If column doesn't exist, add it and set default values
	if count == 0 {
		// Add the column
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN sort_order INTEGER DEFAULT 0`); err != nil {
			return err
		}

		// Set sort_order for existing endpoints based on their current ID order
		if _, err := s.db.Exec(`UPDATE endpoints SET sort_order = id WHERE sort_order = 0`); err != nil {
			return err
		}
	}

	return nil
}

// migrateCacheTokens adds cache token columns to daily_stats table
func (s *SQLiteStorage) migrateCacheTokens() error {
	// Check if cache_creation_tokens column exists
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('daily_stats') WHERE name='cache_creation_tokens'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE daily_stats ADD COLUMN cache_creation_tokens INTEGER DEFAULT 0`); err != nil {
			return err
		}
	}

	// Check if cache_read_tokens column exists
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('daily_stats') WHERE name='cache_read_tokens'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE daily_stats ADD COLUMN cache_read_tokens INTEGER DEFAULT 0`); err != nil {
			return err
		}
	}

	return nil
}

// migrateRequestStats creates the request_stats table
func (s *SQLiteStorage) migrateRequestStats() error {
	// Check if request_stats table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='request_stats'`).Scan(&tableName)

	// Table doesn't exist, create it
	if err == sql.ErrNoRows {
		schema := `
		CREATE TABLE IF NOT EXISTS request_stats (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_name TEXT NOT NULL,
			request_id TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			date TEXT NOT NULL,
			input_tokens INTEGER DEFAULT 0,
			cache_creation_tokens INTEGER DEFAULT 0,
			cache_read_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			model TEXT,
			is_streaming BOOLEAN DEFAULT 0,
			success BOOLEAN DEFAULT 1,
			device_id TEXT DEFAULT 'default',
			error_message TEXT
		);

		CREATE INDEX IF NOT EXISTS idx_request_stats_endpoint ON request_stats(endpoint_name);
		CREATE INDEX IF NOT EXISTS idx_request_stats_date ON request_stats(date);
		CREATE INDEX IF NOT EXISTS idx_request_stats_timestamp ON request_stats(timestamp DESC);
		CREATE INDEX IF NOT EXISTS idx_request_stats_composite ON request_stats(endpoint_name, date, device_id);
		`

		if _, err := s.db.Exec(schema); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

// migrateClientType adds client_type column to endpoints, daily_stats, and request_stats tables
func (s *SQLiteStorage) migrateClientType() error {
	// 1. Add client_type to endpoints table
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='client_type'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Add the column with default 'claude' for existing endpoints
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN client_type TEXT DEFAULT 'claude'`); err != nil {
			return err
		}

		// Drop the old unique constraint on name and create new one on (name, client_type)
		// SQLite doesn't support DROP CONSTRAINT, so we use a unique index instead
		// First, drop the old index if it exists (ignore error if not exists)
		s.db.Exec(`DROP INDEX IF EXISTS idx_endpoints_name`)

		// Create new unique index on (name, client_type)
		if _, err := s.db.Exec(`CREATE UNIQUE INDEX IF NOT EXISTS idx_endpoints_name_client ON endpoints(name, client_type)`); err != nil {
			return err
		}
	}

	// 2. Add client_type to daily_stats table
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('daily_stats') WHERE name='client_type'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE daily_stats ADD COLUMN client_type TEXT DEFAULT 'claude'`); err != nil {
			return err
		}

		// Create index for client_type queries
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_daily_stats_client ON daily_stats(client_type)`); err != nil {
			return err
		}
	}

	// 3. Add client_type to request_stats table
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('request_stats') WHERE name='client_type'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE request_stats ADD COLUMN client_type TEXT DEFAULT 'claude'`); err != nil {
			return err
		}

		// Create index for client_type queries
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_stats_client ON request_stats(client_type)`); err != nil {
			return err
		}
	}

	// 4. Remove the UNIQUE constraint on name column if it exists
	// SQLite creates an auto-index named 'sqlite_autoindex_endpoints_1' for UNIQUE constraints
	// We need to rebuild the table to remove it
	if err := s.migrateRemoveNameUniqueConstraint(); err != nil {
		return err
	}

	// 5. Fix daily_stats unique constraint to include client_type
	if err := s.migrateDailyStatsUniqueConstraint(); err != nil {
		return err
	}

	// 6. Add duration_ms column to request_stats table
	if err := s.migrateDurationTracking(); err != nil {
		return err
	}

	return nil
}

// migrateClientIP adds client_ip column to request_stats table
func (s *SQLiteStorage) migrateClientIP() error {
	// Check if client_ip column exists in request_stats
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('request_stats') WHERE name='client_ip'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Add the column
		if _, err := s.db.Exec(`ALTER TABLE request_stats ADD COLUMN client_ip TEXT DEFAULT ''`); err != nil {
			return err
		}

		// Create index for client_ip queries
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_stats_client_ip ON request_stats(client_ip)`); err != nil {
			return err
		}

		// Create composite index for IP + timestamp queries
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_stats_ip_time ON request_stats(client_ip, timestamp DESC)`); err != nil {
			return err
		}
	}

	return nil
}

// migrateErrorMessage adds error_message column to request_stats table
func (s *SQLiteStorage) migrateErrorMessage() error {
	// Check if error_message column exists in request_stats
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('request_stats') WHERE name='error_message'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Add the column
		if _, err := s.db.Exec(`ALTER TABLE request_stats ADD COLUMN error_message TEXT`); err != nil {
			return err
		}
	}

	return nil
}

// migrateRemoveNameUniqueConstraint removes the UNIQUE constraint on name column
// by rebuilding the endpoints table
func (s *SQLiteStorage) migrateRemoveNameUniqueConstraint() error {
	// Check if the auto-generated unique index exists
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='sqlite_autoindex_endpoints_1'`).Scan(&count)
	if err != nil {
		return err
	}

	// If the auto-index doesn't exist, no migration needed
	if count == 0 {
		return nil
	}

	// SQLite doesn't support dropping constraints directly, so we need to rebuild the table
	// Use a transaction to ensure atomicity
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Create a new table without the UNIQUE constraint on name
	_, err = tx.Exec(`
		CREATE TABLE endpoints_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			name TEXT NOT NULL,
			client_type TEXT DEFAULT 'claude',
			api_url TEXT NOT NULL,
			api_key TEXT NOT NULL,
			enabled BOOLEAN DEFAULT TRUE,
			transformer TEXT DEFAULT 'claude',
			model TEXT,
			remark TEXT,
			sort_order INTEGER DEFAULT 0,
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			updated_at DATETIME DEFAULT CURRENT_TIMESTAMP
		)
	`)
	if err != nil {
		return err
	}

	// 2. Copy data from old table to new table
	_, err = tx.Exec(`
		INSERT INTO endpoints_new (id, name, client_type, api_url, api_key, enabled, transformer, model, remark, sort_order, created_at, updated_at)
		SELECT id, name, COALESCE(client_type, 'claude'), api_url, api_key, enabled, transformer, model, remark, sort_order, created_at, updated_at
		FROM endpoints
	`)
	if err != nil {
		return err
	}

	// 3. Drop the old table
	_, err = tx.Exec(`DROP TABLE endpoints`)
	if err != nil {
		return err
	}

	// 4. Rename new table to endpoints
	_, err = tx.Exec(`ALTER TABLE endpoints_new RENAME TO endpoints`)
	if err != nil {
		return err
	}

	// 5. Recreate the composite unique index on (name, client_type)
	_, err = tx.Exec(`CREATE UNIQUE INDEX idx_endpoints_name_client ON endpoints(name, client_type)`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// migrateDailyStatsUniqueConstraint updates the unique constraint on daily_stats
// to include client_type column
func (s *SQLiteStorage) migrateDailyStatsUniqueConstraint() error {
	// Check if the old unique index exists (without client_type)
	// The auto-generated index for UNIQUE(endpoint_name, date, device_id) is sqlite_autoindex_daily_stats_1
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='sqlite_autoindex_daily_stats_1'`).Scan(&count)
	if err != nil {
		return err
	}

	// If the old auto-index doesn't exist, check if the table needs migration by looking at index info
	if count == 0 {
		// Check if the new correct index exists
		err = s.db.QueryRow(`SELECT COUNT(*) FROM sqlite_master WHERE type='index' AND name='idx_daily_stats_unique'`).Scan(&count)
		if err != nil {
			return err
		}
		if count > 0 {
			// Already migrated
			return nil
		}
		// If neither index exists, check table structure
		// This handles the case where the table was created with the new schema
		return nil
	}

	// Need to rebuild the table to fix the unique constraint
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// 1. Create new table with correct unique constraint
	_, err = tx.Exec(`
		CREATE TABLE daily_stats_new (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_name TEXT NOT NULL,
			client_type TEXT DEFAULT 'claude',
			date TEXT NOT NULL,
			requests INTEGER DEFAULT 0,
			errors INTEGER DEFAULT 0,
			input_tokens INTEGER DEFAULT 0,
			cache_creation_tokens INTEGER DEFAULT 0,
			cache_read_tokens INTEGER DEFAULT 0,
			output_tokens INTEGER DEFAULT 0,
			device_id TEXT DEFAULT 'default',
			created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(endpoint_name, client_type, date, device_id)
		)
	`)
	if err != nil {
		return err
	}

	// 2. Copy data, merging rows that would conflict under the new constraint
	_, err = tx.Exec(`
		INSERT INTO daily_stats_new (endpoint_name, client_type, date, requests, errors, input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens, device_id, created_at)
		SELECT endpoint_name, COALESCE(client_type, 'claude'), date,
			SUM(requests), SUM(errors), SUM(input_tokens),
			SUM(COALESCE(cache_creation_tokens, 0)), SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens),
			device_id, MIN(created_at)
		FROM daily_stats
		GROUP BY endpoint_name, COALESCE(client_type, 'claude'), date, device_id
	`)
	if err != nil {
		return err
	}

	// 3. Drop old table
	_, err = tx.Exec(`DROP TABLE daily_stats`)
	if err != nil {
		return err
	}

	// 4. Rename new table
	_, err = tx.Exec(`ALTER TABLE daily_stats_new RENAME TO daily_stats`)
	if err != nil {
		return err
	}

	// 5. Recreate indexes
	_, err = tx.Exec(`CREATE INDEX idx_daily_stats_date ON daily_stats(date)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE INDEX idx_daily_stats_endpoint ON daily_stats(endpoint_name)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE INDEX idx_daily_stats_device ON daily_stats(device_id)`)
	if err != nil {
		return err
	}
	_, err = tx.Exec(`CREATE INDEX idx_daily_stats_client ON daily_stats(client_type)`)
	if err != nil {
		return err
	}

	return tx.Commit()
}

// migrateDurationTracking adds duration_ms column to request_stats table
func (s *SQLiteStorage) migrateDurationTracking() error {
	// Check if duration_ms column exists
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('request_stats') WHERE name='duration_ms'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		// Add the column with default 0 for existing records
		if _, err := s.db.Exec(`ALTER TABLE request_stats ADD COLUMN duration_ms INTEGER DEFAULT 0`); err != nil {
			return err
		}

		// Create index for duration queries (useful for performance analysis)
		if _, err := s.db.Exec(`CREATE INDEX IF NOT EXISTS idx_request_stats_duration ON request_stats(duration_ms)`); err != nil {
			return err
		}
	}

	return nil
}

func (s *SQLiteStorage) GetEndpoints() ([]Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, name, COALESCE(client_type, 'claude') as client_type, api_url, api_key, enabled, COALESCE(status, '') as status, transformer, model, remark, COALESCE(tags, '') as tags, sort_order, created_at, updated_at, COALESCE(model_patterns, '') as model_patterns, COALESCE(cost_per_input_token, 0) as cost_per_input_token, COALESCE(cost_per_output_token, 0) as cost_per_output_token, COALESCE(quota_limit, 0) as quota_limit, COALESCE(quota_reset_cycle, '') as quota_reset_cycle, COALESCE(priority, 100) as priority FROM endpoints ORDER BY client_type, sort_order ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []Endpoint
	for rows.Next() {
		var ep Endpoint
		var status string
		if err := rows.Scan(&ep.ID, &ep.Name, &ep.ClientType, &ep.APIUrl, &ep.APIKey, &ep.Enabled, &status, &ep.Transformer, &ep.Model, &ep.Remark, &ep.Tags, &ep.SortOrder, &ep.CreatedAt, &ep.UpdatedAt, &ep.ModelPatterns, &ep.CostPerInputToken, &ep.CostPerOutputToken, &ep.QuotaLimit, &ep.QuotaResetCycle, &ep.Priority); err != nil {
			return nil, err
		}
		// 设置状态字段，如果为空则从 enabled 推断
		if status != "" {
			ep.Status = status
		} else {
			if ep.Enabled {
				ep.Status = "untested" // 旧数据迁移：启用的端点设为未检测状态
			} else {
				ep.Status = "disabled"
			}
		}
		endpoints = append(endpoints, ep)
	}

	return endpoints, rows.Err()
}

// GetEndpointsByClient returns endpoints filtered by client type
func (s *SQLiteStorage) GetEndpointsByClient(clientType string) ([]Endpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, name, COALESCE(client_type, 'claude') as client_type, api_url, api_key, enabled, COALESCE(status, '') as status, transformer, model, remark, COALESCE(tags, '') as tags, sort_order, created_at, updated_at, COALESCE(model_patterns, '') as model_patterns, COALESCE(cost_per_input_token, 0) as cost_per_input_token, COALESCE(cost_per_output_token, 0) as cost_per_output_token, COALESCE(quota_limit, 0) as quota_limit, COALESCE(quota_reset_cycle, '') as quota_reset_cycle, COALESCE(priority, 100) as priority FROM endpoints WHERE COALESCE(client_type, 'claude') = ? ORDER BY sort_order ASC`, clientType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []Endpoint
	for rows.Next() {
		var ep Endpoint
		var status string
		if err := rows.Scan(&ep.ID, &ep.Name, &ep.ClientType, &ep.APIUrl, &ep.APIKey, &ep.Enabled, &status, &ep.Transformer, &ep.Model, &ep.Remark, &ep.Tags, &ep.SortOrder, &ep.CreatedAt, &ep.UpdatedAt, &ep.ModelPatterns, &ep.CostPerInputToken, &ep.CostPerOutputToken, &ep.QuotaLimit, &ep.QuotaResetCycle, &ep.Priority); err != nil {
			return nil, err
		}
		// 设置状态字段，如果为空则从 enabled 推断
		if status != "" {
			ep.Status = status
		} else {
			if ep.Enabled {
				ep.Status = "untested" // 旧数据迁移：启用的端点设为未检测状态
			} else {
				ep.Status = "disabled"
			}
		}
		endpoints = append(endpoints, ep)
	}

	return endpoints, rows.Err()
}

func (s *SQLiteStorage) SaveEndpoint(ep *Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Default client_type to 'claude' if not specified
	clientType := ep.ClientType
	if clientType == "" {
		clientType = "claude"
	}

	// Default priority to 100 if not specified
	priority := ep.Priority
	if priority == 0 {
		priority = 100
	}

	result, err := s.db.Exec(`INSERT INTO endpoints (name, client_type, api_url, api_key, enabled, status, transformer, model, remark, tags, sort_order, model_patterns, cost_per_input_token, cost_per_output_token, quota_limit, quota_reset_cycle, priority) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		ep.Name, clientType, ep.APIUrl, ep.APIKey, ep.Enabled, ep.Status, ep.Transformer, ep.Model, ep.Remark, ep.Tags, ep.SortOrder, ep.ModelPatterns, ep.CostPerInputToken, ep.CostPerOutputToken, ep.QuotaLimit, ep.QuotaResetCycle, priority)
	if err != nil {
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	ep.ID = id
	ep.ClientType = clientType
	ep.Priority = priority
	return nil
}

func (s *SQLiteStorage) UpdateEndpoint(ep *Endpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Default client_type to 'claude' if not specified
	clientType := ep.ClientType
	if clientType == "" {
		clientType = "claude"
	}

	// Default priority to 100 if not specified
	priority := ep.Priority
	if priority == 0 {
		priority = 100
	}

	_, err := s.db.Exec(`UPDATE endpoints SET api_url=?, api_key=?, enabled=?, status=?, transformer=?, model=?, remark=?, tags=?, sort_order=?, model_patterns=?, cost_per_input_token=?, cost_per_output_token=?, quota_limit=?, quota_reset_cycle=?, priority=?, updated_at=CURRENT_TIMESTAMP WHERE name=? AND COALESCE(client_type, 'claude')=?`,
		ep.APIUrl, ep.APIKey, ep.Enabled, ep.Status, ep.Transformer, ep.Model, ep.Remark, ep.Tags, ep.SortOrder, ep.ModelPatterns, ep.CostPerInputToken, ep.CostPerOutputToken, ep.QuotaLimit, ep.QuotaResetCycle, priority, ep.Name, clientType)
	return err
}

func (s *SQLiteStorage) DeleteEndpoint(name string, clientType string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Default client_type to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	_, err := s.db.Exec(`DELETE FROM endpoints WHERE name=? AND COALESCE(client_type, 'claude')=?`, name, clientType)
	return err
}

func (s *SQLiteStorage) RecordDailyStat(stat *DailyStat) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Default client_type to 'claude' if not specified
	clientType := stat.ClientType
	if clientType == "" {
		clientType = "claude"
	}

	_, err := s.db.Exec(`
		INSERT INTO daily_stats (endpoint_name, client_type, date, requests, errors, input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens, device_id)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(endpoint_name, client_type, date, device_id) DO UPDATE SET
			requests = requests + excluded.requests,
			errors = errors + excluded.errors,
			input_tokens = input_tokens + excluded.input_tokens,
			cache_creation_tokens = cache_creation_tokens + excluded.cache_creation_tokens,
			cache_read_tokens = cache_read_tokens + excluded.cache_read_tokens,
			output_tokens = output_tokens + excluded.output_tokens
	`, stat.EndpointName, clientType, stat.Date, stat.Requests, stat.Errors, stat.InputTokens, stat.CacheCreationTokens, stat.CacheReadTokens, stat.OutputTokens, stat.DeviceID)

	return err
}

func (s *SQLiteStorage) GetDailyStats(endpointName, clientType, startDate, endDate string) ([]DailyStat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Default client_type to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	query := `SELECT id, endpoint_name, COALESCE(client_type, 'claude') as client_type, date, SUM(requests), SUM(errors),
		SUM(input_tokens), SUM(COALESCE(cache_creation_tokens, 0)), SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens),
		device_id, created_at
		FROM daily_stats WHERE endpoint_name=? AND COALESCE(client_type, 'claude')=? AND date>=? AND date<=? GROUP BY date ORDER BY date DESC`

	rows, err := s.db.Query(query, endpointName, clientType, startDate, endDate)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []DailyStat
	for rows.Next() {
		var stat DailyStat
		if err := rows.Scan(&stat.ID, &stat.EndpointName, &stat.ClientType, &stat.Date, &stat.Requests, &stat.Errors,
			&stat.InputTokens, &stat.CacheCreationTokens, &stat.CacheReadTokens, &stat.OutputTokens,
			&stat.DeviceID, &stat.CreatedAt); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	return stats, rows.Err()
}

func (s *SQLiteStorage) GetAllStats() (map[string][]DailyStat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT id, endpoint_name, COALESCE(client_type, 'claude') as client_type, date, SUM(requests), SUM(errors),
		SUM(input_tokens), SUM(COALESCE(cache_creation_tokens, 0)), SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens),
		device_id, created_at
		FROM daily_stats GROUP BY endpoint_name, client_type, date ORDER BY date DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	result := make(map[string][]DailyStat)
	for rows.Next() {
		var stat DailyStat
		if err := rows.Scan(&stat.ID, &stat.EndpointName, &stat.ClientType, &stat.Date, &stat.Requests, &stat.Errors,
			&stat.InputTokens, &stat.CacheCreationTokens, &stat.CacheReadTokens, &stat.OutputTokens,
			&stat.DeviceID, &stat.CreatedAt); err != nil {
			return nil, err
		}
		// Use clientType:endpointName as key to distinguish endpoints across client types
		key := stat.ClientType + ":" + stat.EndpointName
		result[key] = append(result[key], stat)
	}

	return result, rows.Err()
}

func (s *SQLiteStorage) GetConfig(key string) (string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var value string
	err := s.db.QueryRow(`SELECT value FROM app_config WHERE key=?`, key).Scan(&value)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return value, err
}

func (s *SQLiteStorage) SetConfig(key, value string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`INSERT INTO app_config (key, value) VALUES (?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value, updated_at=CURRENT_TIMESTAMP`, key, value)
	return err
}

func (s *SQLiteStorage) Close() error {
	return s.db.Close()
}

func (s *SQLiteStorage) GetTotalStats() (int, map[string]*EndpointStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT COALESCE(client_type, 'claude') as client_type, endpoint_name, SUM(requests), SUM(errors),
		SUM(input_tokens), SUM(COALESCE(cache_creation_tokens, 0)), SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens)
		FROM daily_stats GROUP BY client_type, endpoint_name`

	rows, err := s.db.Query(query)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	result := make(map[string]*EndpointStats)
	totalRequests := 0

	for rows.Next() {
		var clientType, endpointName string
		var requests, errors int
		var inputTokens, cacheCreationTokens, cacheReadTokens, outputTokens int64

		if err := rows.Scan(&clientType, &endpointName, &requests, &errors,
			&inputTokens, &cacheCreationTokens, &cacheReadTokens, &outputTokens); err != nil {
			return 0, nil, err
		}

		// Use clientType:endpointName as key
		key := clientType + ":" + endpointName
		result[key] = &EndpointStats{
			Requests:            requests,
			Errors:              errors,
			InputTokens:         inputTokens,
			CacheCreationTokens: cacheCreationTokens,
			CacheReadTokens:     cacheReadTokens,
			OutputTokens:        outputTokens,
		}
		totalRequests += requests
	}

	return totalRequests, result, rows.Err()
}

// GetTotalStatsByClient returns total stats filtered by client type
func (s *SQLiteStorage) GetTotalStatsByClient(clientType string) (int, map[string]*EndpointStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Default client_type to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	query := `SELECT endpoint_name, SUM(requests), SUM(errors),
		SUM(input_tokens), SUM(COALESCE(cache_creation_tokens, 0)), SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens)
		FROM daily_stats WHERE COALESCE(client_type, 'claude') = ? GROUP BY endpoint_name`

	rows, err := s.db.Query(query, clientType)
	if err != nil {
		return 0, nil, err
	}
	defer rows.Close()

	result := make(map[string]*EndpointStats)
	totalRequests := 0

	for rows.Next() {
		var endpointName string
		var requests, errors int
		var inputTokens, cacheCreationTokens, cacheReadTokens, outputTokens int64

		if err := rows.Scan(&endpointName, &requests, &errors,
			&inputTokens, &cacheCreationTokens, &cacheReadTokens, &outputTokens); err != nil {
			return 0, nil, err
		}

		result[endpointName] = &EndpointStats{
			Requests:            requests,
			Errors:              errors,
			InputTokens:         inputTokens,
			CacheCreationTokens: cacheCreationTokens,
			CacheReadTokens:     cacheReadTokens,
			OutputTokens:        outputTokens,
		}
		totalRequests += requests
	}

	return totalRequests, result, rows.Err()
}

func (s *SQLiteStorage) GetEndpointTotalStats(endpointName string, clientType string) (*EndpointStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Default client_type to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	query := `SELECT SUM(requests), SUM(errors),
		SUM(input_tokens), SUM(COALESCE(cache_creation_tokens, 0)), SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens)
		FROM daily_stats WHERE endpoint_name=? AND COALESCE(client_type, 'claude')=?`

	var requests, errors int
	var inputTokens, cacheCreationTokens, cacheReadTokens, outputTokens int64

	err := s.db.QueryRow(query, endpointName, clientType).Scan(&requests, &errors,
		&inputTokens, &cacheCreationTokens, &cacheReadTokens, &outputTokens)
	if err == sql.ErrNoRows {
		return &EndpointStats{}, nil
	}
	if err != nil {
		return nil, err
	}

	return &EndpointStats{
		Requests:            requests,
		Errors:              errors,
		InputTokens:         inputTokens,
		CacheCreationTokens: cacheCreationTokens,
		CacheReadTokens:     cacheReadTokens,
		OutputTokens:        outputTokens,
	}, nil
}

// GetOrCreateDeviceID returns the device ID, creating one if it doesn't exist
func (s *SQLiteStorage) GetOrCreateDeviceID() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Try to get existing device ID
	var deviceID string
	err := s.db.QueryRow(`SELECT value FROM app_config WHERE key = 'device_id'`).Scan(&deviceID)

	if err == nil && deviceID != "" {
		return deviceID, nil
	}

	// Generate new device ID
	deviceID = generateDeviceID()

	// Save to database
	_, err = s.db.Exec(`INSERT OR REPLACE INTO app_config (key, value) VALUES ('device_id', ?)`, deviceID)
	if err != nil {
		return "", err
	}

	return deviceID, nil
}

func generateDeviceID() string {
	// Use timestamp + random string for uniqueness
	timestamp := time.Now().UnixNano()
	return fmt.Sprintf("device-%x", timestamp)[:16]
}

func GenerateDeviceID() string {
	return generateDeviceID()
}

// GetDBPath returns the database file path
func (s *SQLiteStorage) GetDBPath() string {
	return s.dbPath
}

// GetArchiveMonths returns a list of all months that have data
func (s *SQLiteStorage) GetArchiveMonths() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT DISTINCT strftime('%Y-%m', date) as month
		FROM daily_stats
		WHERE date IS NOT NULL AND date != ''
		ORDER BY month DESC`

	rows, err := s.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var months []string
	for rows.Next() {
		var month string
		if err := rows.Scan(&month); err != nil {
			return nil, err
		}
		months = append(months, month)
	}

	return months, rows.Err()
}

// MonthlyArchiveData represents archive data for a specific month
type MonthlyArchiveData struct {
	Month               string
	EndpointName        string
	ClientType          string
	Date                string
	Requests            int
	Errors              int
	InputTokens         int
	CacheCreationTokens int
	CacheReadTokens     int
	OutputTokens        int
}

// GetMonthlyArchiveData returns all daily stats for a specific month
func (s *SQLiteStorage) GetMonthlyArchiveData(month string) ([]MonthlyArchiveData, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	query := `SELECT endpoint_name, COALESCE(client_type, 'claude') as client_type, date, SUM(requests), SUM(errors),
		SUM(input_tokens), SUM(COALESCE(cache_creation_tokens, 0)), SUM(COALESCE(cache_read_tokens, 0)), SUM(output_tokens)
		FROM daily_stats
		WHERE strftime('%Y-%m', date) = ?
		GROUP BY endpoint_name, client_type, date
		ORDER BY date DESC, client_type, endpoint_name`

	rows, err := s.db.Query(query, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var results []MonthlyArchiveData
	for rows.Next() {
		var data MonthlyArchiveData
		data.Month = month
		if err := rows.Scan(&data.EndpointName, &data.ClientType, &data.Date, &data.Requests, &data.Errors,
			&data.InputTokens, &data.CacheCreationTokens, &data.CacheReadTokens, &data.OutputTokens); err != nil {
			return nil, err
		}
		results = append(results, data)
	}

	return results, rows.Err()
}

// DeleteMonthlyStats deletes all daily stats for a specific month
func (s *SQLiteStorage) DeleteMonthlyStats(month string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	_, err := s.db.Exec(`DELETE FROM daily_stats WHERE strftime('%Y-%m', date) = ?`, month)
	return err
}

// CreateBackupCopy 创建数据库备份副本，只保留安全的 app_config 配置项。
// 设备特定的配置（device_id、终端设置、本地路径等）会被排除。
func (s *SQLiteStorage) CreateBackupCopy(backupPath string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 使用 VACUUM INTO 创建数据库副本
	_, err := s.db.Exec(fmt.Sprintf("VACUUM INTO '%s'", backupPath))
	if err != nil {
		return fmt.Errorf("failed to create backup: %w", err)
	}

	// 打开备份数据库并清理设备特定的 app_config 数据
	backupDB, err := sql.Open("sqlite", backupPath)
	if err != nil {
		return fmt.Errorf("failed to open backup: %w", err)
	}
	defer backupDB.Close()

	// 删除所有不在安全列表中的 app_config 条目
	// 这会移除 device_id、terminal_*、backup_local_dir、proxy_url、windowWidth/Height 等
	placeholders := make([]string, len(safeConfigKeys))
	args := make([]interface{}, len(safeConfigKeys))
	for i, key := range safeConfigKeys {
		placeholders[i] = "?"
		args[i] = key
	}
	query := fmt.Sprintf("DELETE FROM app_config WHERE key NOT IN (%s)", strings.Join(placeholders, ","))
	_, err = backupDB.Exec(query, args...)
	if err != nil {
		return fmt.Errorf("failed to clean app_config: %w", err)
	}

	return nil
}

// MergeConflict represents an endpoint merge conflict
type MergeConflict struct {
	EndpointName   string   `json:"endpointName"`
	ConflictFields []string `json:"conflictFields"`
	LocalEndpoint  Endpoint `json:"localEndpoint"`
	RemoteEndpoint Endpoint `json:"remoteEndpoint"`
}

// DetectEndpointConflicts detects conflicts between local and remote endpoints
func (s *SQLiteStorage) DetectEndpointConflicts(remoteDBPath string) ([]MergeConflict, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Attach remote database
	_, err := s.db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS remote", remoteDBPath))
	if err != nil {
		return nil, fmt.Errorf("failed to attach remote database: %w", err)
	}
	defer s.db.Exec("DETACH DATABASE remote")

	// Get local endpoints
	localEndpoints, err := s.getEndpointsFromDB(s.db, "main")
	if err != nil {
		return nil, err
	}

	// Get remote endpoints
	remoteEndpoints, err := s.getEndpointsFromDB(s.db, "remote")
	if err != nil {
		return nil, err
	}

	// Build local endpoint map using composite key (clientType:name)
	localMap := make(map[string]Endpoint)
	for _, ep := range localEndpoints {
		clientType := ep.ClientType
		if clientType == "" {
			clientType = "claude"
		}
		key := clientType + ":" + ep.Name
		localMap[key] = ep
	}

	// Detect conflicts
	var conflicts []MergeConflict
	for _, remote := range remoteEndpoints {
		remoteClientType := remote.ClientType
		if remoteClientType == "" {
			remoteClientType = "claude"
		}
		key := remoteClientType + ":" + remote.Name
		if local, exists := localMap[key]; exists {
			// Check for differences (same client type, same name)
			conflictFields := compareEndpoints(local, remote)
			if len(conflictFields) > 0 {
				conflicts = append(conflicts, MergeConflict{
					EndpointName:   remote.Name,
					ConflictFields: conflictFields,
					LocalEndpoint:  local,
					RemoteEndpoint: remote,
				})
			}
		}
	}

	return conflicts, nil
}

// getEndpointsFromDB gets endpoints from a specific database (main or attached)
func (s *SQLiteStorage) getEndpointsFromDB(db *sql.DB, dbName string) ([]Endpoint, error) {
	query := fmt.Sprintf(`SELECT id, name, COALESCE(client_type, 'claude') as client_type, api_url, api_key, enabled, transformer, model, remark, COALESCE(tags, '') as tags, COALESCE(sort_order, 0) as sort_order, created_at, updated_at FROM %s.endpoints`, dbName)
	rows, err := db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var endpoints []Endpoint
	for rows.Next() {
		var ep Endpoint
		if err := rows.Scan(&ep.ID, &ep.Name, &ep.ClientType, &ep.APIUrl, &ep.APIKey, &ep.Enabled, &ep.Transformer, &ep.Model, &ep.Remark, &ep.Tags, &ep.SortOrder, &ep.CreatedAt, &ep.UpdatedAt); err != nil {
			return nil, err
		}
		endpoints = append(endpoints, ep)
	}

	return endpoints, rows.Err()
}

// compareEndpoints compares two endpoints and returns conflicting fields
func compareEndpoints(local, remote Endpoint) []string {
	var conflicts []string

	if local.APIUrl != remote.APIUrl {
		conflicts = append(conflicts, "apiUrl")
	}
	if local.APIKey != remote.APIKey {
		conflicts = append(conflicts, "apiKey")
	}
	if local.Enabled != remote.Enabled {
		conflicts = append(conflicts, "enabled")
	}
	if local.Transformer != remote.Transformer {
		conflicts = append(conflicts, "transformer")
	}
	if local.Model != remote.Model {
		conflicts = append(conflicts, "model")
	}
	if local.Remark != remote.Remark {
		conflicts = append(conflicts, "remark")
	}
	if local.Tags != remote.Tags {
		conflicts = append(conflicts, "tags")
	}

	return conflicts
}

// MergeStrategy 定义合并时如何处理冲突
type MergeStrategy string

const (
	MergeStrategyKeepLocal      MergeStrategy = "keep_local"      // 冲突时保留本地，添加新数据
	MergeStrategyOverwriteLocal MergeStrategy = "overwrite_local" // 冲突时用备份覆盖本地
)

// MergeFromBackup 从备份数据库合并数据
func (s *SQLiteStorage) MergeFromBackup(backupDBPath string, strategy MergeStrategy) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 挂载备份数据库
	_, err := s.db.Exec(fmt.Sprintf("ATTACH DATABASE '%s' AS backup", backupDBPath))
	if err != nil {
		return fmt.Errorf("failed to attach backup database: %w", err)
	}
	defer s.db.Exec("DETACH DATABASE backup")

	// 开启事务
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	// 1. 根据策略合并端点配置
	if err := s.mergeEndpoints(tx, strategy); err != nil {
		return fmt.Errorf("failed to merge endpoints: %w", err)
	}

	// 2. 根据策略合并每日统计数据
	if err := s.mergeDailyStats(tx, strategy); err != nil {
		return fmt.Errorf("failed to merge daily stats: %w", err)
	}

	// 3. 合并安全的 app_config 配置项（仅平台无关的设置）
	if err := s.mergeAppConfig(tx, strategy); err != nil {
		return fmt.Errorf("failed to merge app config: %w", err)
	}

	// 提交事务
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit transaction: %w", err)
	}

	return nil
}

// mergeEndpoints 根据策略合并端点配置
func (s *SQLiteStorage) mergeEndpoints(tx *sql.Tx, strategy MergeStrategy) error {
	switch strategy {
	case MergeStrategyKeepLocal:
		// 只插入新端点（忽略冲突）
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO endpoints
			(name, client_type, api_url, api_key, enabled, transformer, model, remark, tags, sort_order)
			SELECT name, COALESCE(client_type, 'claude'), api_url, api_key, enabled, transformer, model, remark, COALESCE(tags, ''), COALESCE(sort_order, 0)
			FROM backup.endpoints
		`)
		return err
	case MergeStrategyOverwriteLocal:
		// 替换已存在的端点
		_, err := tx.Exec(`
			INSERT OR REPLACE INTO endpoints
			(name, client_type, api_url, api_key, enabled, transformer, model, remark, tags, sort_order)
			SELECT name, COALESCE(client_type, 'claude'), api_url, api_key, enabled, transformer, model, remark, COALESCE(tags, ''), COALESCE(sort_order, 0)
			FROM backup.endpoints
		`)
		return err
	default:
		return fmt.Errorf("unknown merge strategy: %s", strategy)
	}
}

// mergeDailyStats 根据策略合并每日统计数据
// 注意：备份数据的 device_id 会被替换为本地的 device_id，以避免跨设备恢复时产生重复记录
func (s *SQLiteStorage) mergeDailyStats(tx *sql.Tx, strategy MergeStrategy) error {
	// 获取本地 device_id，如果不存在则使用 'default'
	var localDeviceID string
	err := tx.QueryRow(`SELECT COALESCE((SELECT value FROM app_config WHERE key = 'device_id'), 'default')`).Scan(&localDeviceID)
	if err != nil {
		localDeviceID = "default"
	}

	switch strategy {
	case MergeStrategyKeepLocal:
		// 保留本地数据，只插入本地不存在的记录
		// 使用本地 device_id 替代备份的 device_id 以避免重复
		_, err := tx.Exec(`
			INSERT OR IGNORE INTO daily_stats
			(endpoint_name, client_type, date, requests, errors, input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens, device_id)
			SELECT endpoint_name, COALESCE(client_type, 'claude'), date, requests, errors, input_tokens,
				COALESCE(cache_creation_tokens, 0), COALESCE(cache_read_tokens, 0), output_tokens, ?
			FROM backup.daily_stats
		`, localDeviceID)
		return err
	case MergeStrategyOverwriteLocal:
		// 用备份数据覆盖本地数据
		// 步骤1：删除主数据库中的冲突记录（匹配 endpoint_name, client_type 和 date）
		_, err := tx.Exec(`
			DELETE FROM daily_stats
			WHERE EXISTS (
				SELECT 1 FROM backup.daily_stats b
				WHERE b.endpoint_name = daily_stats.endpoint_name
				AND COALESCE(b.client_type, 'claude') = COALESCE(daily_stats.client_type, 'claude')
				AND b.date = daily_stats.date
			)
		`)
		if err != nil {
			return err
		}

		// 步骤2：使用本地 device_id 插入备份数据
		_, err = tx.Exec(`
			INSERT INTO daily_stats
			(endpoint_name, client_type, date, requests, errors, input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens, device_id)
			SELECT endpoint_name, COALESCE(client_type, 'claude'), date, requests, errors, input_tokens,
				COALESCE(cache_creation_tokens, 0), COALESCE(cache_read_tokens, 0), output_tokens, ?
			FROM backup.daily_stats
		`, localDeviceID)
		return err
	default:
		return fmt.Errorf("unknown merge strategy: %s", strategy)
	}
}

// mergeAppConfig 根据策略合并安全的 app_config 配置项
// 只有 safeConfigKeys 中的配置会被合并；设备特定的配置会保留本地值
func (s *SQLiteStorage) mergeAppConfig(tx *sql.Tx, strategy MergeStrategy) error {
	// 构建安全配置项的占位符
	placeholders := make([]string, len(safeConfigKeys))
	args := make([]interface{}, len(safeConfigKeys))
	for i, key := range safeConfigKeys {
		placeholders[i] = "?"
		args[i] = key
	}
	keysFilter := strings.Join(placeholders, ",")

	switch strategy {
	case MergeStrategyKeepLocal:
		// 保留本地值，只插入备份中新增的配置项
		query := fmt.Sprintf(`
			INSERT OR IGNORE INTO app_config (key, value)
			SELECT key, value FROM backup.app_config
			WHERE key IN (%s)
		`, keysFilter)
		_, err := tx.Exec(query, args...)
		return err
	case MergeStrategyOverwriteLocal:
		// 用备份值覆盖本地值（仅限安全配置项）
		query := fmt.Sprintf(`
			INSERT OR REPLACE INTO app_config (key, value)
			SELECT key, value FROM backup.app_config
			WHERE key IN (%s)
		`, keysFilter)
		_, err := tx.Exec(query, args...)
		return err
	default:
		return fmt.Errorf("unknown merge strategy: %s", strategy)
	}
}

// RecordRequestStat records a single request-level statistic
func (s *SQLiteStorage) RecordRequestStat(stat *RequestStat) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Default client_type to 'claude' if not specified
	clientType := stat.ClientType
	if clientType == "" {
		clientType = "claude"
	}

	// Limit error message length to 500 characters
	errorMessage := stat.ErrorMessage
	if len(errorMessage) > 500 {
		errorMessage = errorMessage[:500]
	}

	_, err := s.db.Exec(`
		INSERT INTO request_stats (
			endpoint_name, client_type, client_ip, request_id, timestamp, date,
			input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens,
			model, is_streaming, success, device_id, duration_ms, error_message
		)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`,
		stat.EndpointName,        // endpoint_name
		clientType,               // client_type
		stat.ClientIP,            // client_ip
		stat.RequestID,           // request_id
		stat.Timestamp,           // timestamp
		stat.Date,                // date
		stat.InputTokens,         // input_tokens
		stat.CacheCreationTokens, // cache_creation_tokens
		stat.CacheReadTokens,     // cache_read_tokens
		stat.OutputTokens,        // output_tokens
		stat.Model,               // model
		stat.IsStreaming,         // is_streaming
		stat.Success,             // success
		stat.DeviceID,            // device_id
		stat.DurationMs,          // duration_ms
		errorMessage,             // error_message
	)

	return err
}

// GetRequestStats retrieves request-level statistics with pagination
func (s *SQLiteStorage) GetRequestStats(endpointName string, clientType string, startDate, endDate string, limit, offset int) ([]RequestStat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Default client_type to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	var query string
	var args []interface{}

	if endpointName == "" {
		// Query all endpoints for this client type
		query = `
			SELECT id, endpoint_name, COALESCE(client_type, 'claude') as client_type, COALESCE(client_ip, '') as client_ip,
				request_id, timestamp, date,
				input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens,
				model, is_streaming, success, device_id, COALESCE(duration_ms, 0) as duration_ms,
				COALESCE(error_message, '') as error_message
			FROM request_stats
			WHERE COALESCE(client_type, 'claude')=? AND date>=? AND date<=?
			ORDER BY timestamp DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{clientType, startDate, endDate, limit, offset}
	} else {
		// Query specific endpoint
		query = `
			SELECT id, endpoint_name, COALESCE(client_type, 'claude') as client_type, COALESCE(client_ip, '') as client_ip,
				request_id, timestamp, date,
				input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens,
				model, is_streaming, success, device_id, COALESCE(duration_ms, 0) as duration_ms,
				COALESCE(error_message, '') as error_message
			FROM request_stats
			WHERE endpoint_name=? AND COALESCE(client_type, 'claude')=? AND date>=? AND date<=?
			ORDER BY timestamp DESC
			LIMIT ? OFFSET ?
		`
		args = []interface{}{endpointName, clientType, startDate, endDate, limit, offset}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []RequestStat
	for rows.Next() {
		var stat RequestStat
		if err := rows.Scan(
			&stat.ID, &stat.EndpointName, &stat.ClientType, &stat.ClientIP,
			&stat.RequestID, &stat.Timestamp, &stat.Date,
			&stat.InputTokens, &stat.CacheCreationTokens, &stat.CacheReadTokens, &stat.OutputTokens,
			&stat.Model, &stat.IsStreaming, &stat.Success, &stat.DeviceID, &stat.DurationMs,
			&stat.ErrorMessage,
		); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	return stats, rows.Err()
}

// GetRequestStatsCount gets the total count of request stats for pagination
func (s *SQLiteStorage) GetRequestStatsCount(endpointName string, clientType string, startDate, endDate string) (int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Default client_type to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	var count int
	var err error

	if endpointName == "" {
		// Count all endpoints for this client type
		err = s.db.QueryRow(`
			SELECT COUNT(*) FROM request_stats
			WHERE COALESCE(client_type, 'claude')=? AND date>=? AND date<=?
		`, clientType, startDate, endDate).Scan(&count)
	} else {
		// Count specific endpoint
		err = s.db.QueryRow(`
			SELECT COUNT(*) FROM request_stats
			WHERE endpoint_name=? AND COALESCE(client_type, 'claude')=? AND date>=? AND date<=?
		`, endpointName, clientType, startDate, endDate).Scan(&count)
	}

	return count, err
}

// CleanupOldRequestStats deletes request stats older than specified days
func (s *SQLiteStorage) CleanupOldRequestStats(daysToKeep int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoffDate := time.Now().AddDate(0, 0, -daysToKeep).Format("2006-01-02")

	result, err := s.db.Exec(`
		DELETE FROM request_stats WHERE date < ?
	`, cutoffDate)

	if err != nil {
		return err
	}

	rowsAffected, _ := result.RowsAffected()
	if rowsAffected > 0 {
		// Note: logger is not available in this package, would need to be passed in or logged at a higher level
		// For now, we'll just return success
	}

	return nil
}

// GetConnectedClients returns clients that have made requests in the past N hours
func (s *SQLiteStorage) GetConnectedClients(hoursAgo int) ([]ClientStats, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	cutoffTime := time.Now().Add(-time.Duration(hoursAgo) * time.Hour)

	query := `
		SELECT
			client_ip,
			MAX(timestamp) as last_seen,
			COUNT(*) as request_count,
			SUM(input_tokens) as input_tokens,
			SUM(cache_creation_tokens) as cache_creation_tokens,
			SUM(cache_read_tokens) as cache_read_tokens,
			SUM(output_tokens) as output_tokens,
			GROUP_CONCAT(DISTINCT endpoint_name) as endpoints
		FROM request_stats
		WHERE client_ip != '' AND client_ip IS NOT NULL AND timestamp >= ?
		GROUP BY client_ip
		ORDER BY last_seen DESC
	`

	rows, err := s.db.Query(query, cutoffTime)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var clients []ClientStats
	for rows.Next() {
		var c ClientStats
		var lastSeenStr string
		var endpointsStr sql.NullString
		if err := rows.Scan(
			&c.ClientIP, &lastSeenStr, &c.RequestCount,
			&c.InputTokens, &c.CacheCreationTokens, &c.CacheReadTokens, &c.OutputTokens,
			&endpointsStr,
		); err != nil {
			return nil, err
		}

		// Parse last_seen time from SQLite string format
		if lastSeenStr != "" {
			// Try parsing with common SQLite datetime formats
			formats := []string{
				"2006-01-02 15:04:05",           // SQLite default DATETIME format
				"2006-01-02T15:04:05Z",          // ISO 8601 UTC
				"2006-01-02T15:04:05.999999999", // With nanoseconds
				time.RFC3339,                    // RFC3339 format
			}

			var parseErr error
			for _, format := range formats {
				c.LastSeen, parseErr = time.Parse(format, lastSeenStr)
				if parseErr == nil {
					break
				}
			}
			// If all formats fail, use current time as fallback
			if parseErr != nil {
				c.LastSeen = time.Now()
			}
		} else {
			c.LastSeen = time.Now()
		}

		if endpointsStr.Valid && endpointsStr.String != "" {
			c.EndpointsUsed = strings.Split(endpointsStr.String, ",")
		} else {
			c.EndpointsUsed = []string{}
		}
		clients = append(clients, c)
	}

	return clients, rows.Err()
}

// migrateEndpointTags adds the tags column to endpoints table
func (s *SQLiteStorage) migrateEndpointTags() error {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='tags'`).Scan(&count)
	if err != nil {
		return err
	}

	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN tags TEXT DEFAULT ''`); err != nil {
			return err
		}
	}

	return nil
}

// migrateHealthHistory creates the endpoint_health_history table
func (s *SQLiteStorage) migrateHealthHistory() error {
	// Check if table exists
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='endpoint_health_history'`).Scan(&tableName)

	if err == sql.ErrNoRows {
		schema := `
		CREATE TABLE IF NOT EXISTS endpoint_health_history (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_name TEXT NOT NULL,
			client_type TEXT DEFAULT 'claude',
			status TEXT NOT NULL,
			latency_ms REAL,
			error_message TEXT,
			timestamp DATETIME DEFAULT CURRENT_TIMESTAMP,
			device_id TEXT DEFAULT 'default'
		);

		CREATE INDEX IF NOT EXISTS idx_health_history_endpoint ON endpoint_health_history(endpoint_name, client_type);
		CREATE INDEX IF NOT EXISTS idx_health_history_timestamp ON endpoint_health_history(timestamp DESC);
		`

		if _, err := s.db.Exec(schema); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

// RecordHealthHistory records a health check result to history
func (s *SQLiteStorage) RecordHealthHistory(record *HealthHistoryRecord) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clientType := record.ClientType
	if clientType == "" {
		clientType = "claude"
	}

	_, err := s.db.Exec(`
		INSERT INTO endpoint_health_history (endpoint_name, client_type, status, latency_ms, error_message, timestamp, device_id)
		VALUES (?, ?, ?, ?, ?, ?, ?)
	`, record.EndpointName, clientType, record.Status, record.LatencyMs, record.ErrorMessage, record.Timestamp, record.DeviceID)

	return err
}

// GetHealthHistory retrieves health history for an endpoint within a time range
func (s *SQLiteStorage) GetHealthHistory(endpointName, clientType string, startTime, endTime time.Time, limit int) ([]HealthHistoryRecord, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if clientType == "" {
		clientType = "claude"
	}

	var query string
	var args []interface{}

	if endpointName == "" {
		// Get all endpoints
		query = `
			SELECT id, endpoint_name, COALESCE(client_type, 'claude') as client_type, status, latency_ms, COALESCE(error_message, '') as error_message, timestamp, device_id
			FROM endpoint_health_history
			WHERE COALESCE(client_type, 'claude') = ? AND timestamp >= ? AND timestamp <= ?
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{clientType, startTime, endTime, limit}
	} else {
		query = `
			SELECT id, endpoint_name, COALESCE(client_type, 'claude') as client_type, status, latency_ms, COALESCE(error_message, '') as error_message, timestamp, device_id
			FROM endpoint_health_history
			WHERE endpoint_name = ? AND COALESCE(client_type, 'claude') = ? AND timestamp >= ? AND timestamp <= ?
			ORDER BY timestamp DESC
			LIMIT ?
		`
		args = []interface{}{endpointName, clientType, startTime, endTime, limit}
	}

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var records []HealthHistoryRecord
	for rows.Next() {
		var r HealthHistoryRecord
		var timestampStr string
		if err := rows.Scan(&r.ID, &r.EndpointName, &r.ClientType, &r.Status, &r.LatencyMs, &r.ErrorMessage, &timestampStr, &r.DeviceID); err != nil {
			return nil, err
		}

		// Parse timestamp
		formats := []string{
			"2006-01-02 15:04:05",
			"2006-01-02T15:04:05Z",
			"2006-01-02T15:04:05.999999999",
			time.RFC3339,
		}
		for _, format := range formats {
			if t, err := time.Parse(format, timestampStr); err == nil {
				r.Timestamp = t
				break
			}
		}

		records = append(records, r)
	}

	return records, rows.Err()
}

// CleanupOldHealthHistory removes health history records older than specified days
func (s *SQLiteStorage) CleanupOldHealthHistory(daysToKeep int) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	cutoffTime := time.Now().AddDate(0, 0, -daysToKeep)

	_, err := s.db.Exec(`DELETE FROM endpoint_health_history WHERE timestamp < ?`, cutoffTime)
	return err
}

// GetAllEndpointTags returns all unique tags used across all endpoints
func (s *SQLiteStorage) GetAllEndpointTags() ([]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	rows, err := s.db.Query(`SELECT DISTINCT tags FROM endpoints WHERE tags != '' AND tags IS NOT NULL`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	tagSet := make(map[string]bool)
	for rows.Next() {
		var tags string
		if err := rows.Scan(&tags); err != nil {
			return nil, err
		}
		// Split comma-separated tags
		for _, tag := range strings.Split(tags, ",") {
			tag = strings.TrimSpace(tag)
			if tag != "" {
				tagSet[tag] = true
			}
		}
	}

	// Convert to slice
	var result []string
	for tag := range tagSet {
		result = append(result, tag)
	}

	return result, rows.Err()
}

// migrateRoutingFields adds routing-related columns to endpoints table
func (s *SQLiteStorage) migrateRoutingFields() error {
	// 检查并添加 model_patterns 列
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='model_patterns'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN model_patterns TEXT DEFAULT ''`); err != nil {
			return err
		}
	}

	// 检查并添加 cost_per_input_token 列
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='cost_per_input_token'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN cost_per_input_token REAL DEFAULT 0`); err != nil {
			return err
		}
	}

	// 检查并添加 cost_per_output_token 列
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='cost_per_output_token'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN cost_per_output_token REAL DEFAULT 0`); err != nil {
			return err
		}
	}

	// 检查并添加 quota_limit 列
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='quota_limit'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN quota_limit INTEGER DEFAULT 0`); err != nil {
			return err
		}
	}

	// 检查并添加 quota_reset_cycle 列
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='quota_reset_cycle'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN quota_reset_cycle TEXT DEFAULT ''`); err != nil {
			return err
		}
	}

	// 检查并添加 priority 列
	err = s.db.QueryRow(`SELECT COUNT(*) FROM pragma_table_info('endpoints') WHERE name='priority'`).Scan(&count)
	if err != nil {
		return err
	}
	if count == 0 {
		if _, err := s.db.Exec(`ALTER TABLE endpoints ADD COLUMN priority INTEGER DEFAULT 100`); err != nil {
			return err
		}
	}

	return nil
}

// migrateEndpointQuotas creates the endpoint_quotas table
func (s *SQLiteStorage) migrateEndpointQuotas() error {
	// 检查表是否存在
	var tableName string
	err := s.db.QueryRow(`SELECT name FROM sqlite_master WHERE type='table' AND name='endpoint_quotas'`).Scan(&tableName)

	if err == sql.ErrNoRows {
		schema := `
		CREATE TABLE IF NOT EXISTS endpoint_quotas (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			endpoint_name TEXT NOT NULL,
			client_type TEXT DEFAULT 'claude',
			period_start DATETIME NOT NULL,
			period_end DATETIME NOT NULL,
			tokens_used INTEGER DEFAULT 0,
			quota_limit INTEGER DEFAULT 0,
			last_updated DATETIME DEFAULT CURRENT_TIMESTAMP,
			UNIQUE(endpoint_name, client_type, period_start)
		);

		CREATE INDEX IF NOT EXISTS idx_endpoint_quotas_name ON endpoint_quotas(endpoint_name, client_type);
		CREATE INDEX IF NOT EXISTS idx_endpoint_quotas_period ON endpoint_quotas(period_end);
		`

		if _, err := s.db.Exec(schema); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}

	return nil
}

// migrateEndpointStatus 添加端点状态字段并迁移现有数据
func (s *SQLiteStorage) migrateEndpointStatus() error {
	// 检查 status 列是否存在
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM pragma_table_info('endpoints')
		WHERE name='status'
	`).Scan(&count)
	if err != nil {
		return err
	}

	if count > 0 {
		return nil // 已迁移
	}

	// 添加 status 列
	_, err = s.db.Exec(`ALTER TABLE endpoints ADD COLUMN status TEXT DEFAULT 'available'`)
	if err != nil {
		return err
	}

	// 迁移现有数据: enabled=true → available, enabled=false → disabled
	_, err = s.db.Exec(`
		UPDATE endpoints
		SET status = CASE
			WHEN enabled = 1 THEN 'available'
			ELSE 'disabled'
		END
	`)
	if err != nil {
		return err
	}

	return nil
}

// GetEndpointQuota gets the quota record for an endpoint
func (s *SQLiteStorage) GetEndpointQuota(endpointName, clientType string) (*EndpointQuota, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if clientType == "" {
		clientType = "claude"
	}

	var quota EndpointQuota
	var periodStartStr, periodEndStr, lastUpdatedStr string

	err := s.db.QueryRow(`
		SELECT id, endpoint_name, COALESCE(client_type, 'claude') as client_type,
			   period_start, period_end, tokens_used, quota_limit, last_updated
		FROM endpoint_quotas
		WHERE endpoint_name = ? AND COALESCE(client_type, 'claude') = ?
		ORDER BY period_start DESC
		LIMIT 1
	`, endpointName, clientType).Scan(
		&quota.ID, &quota.EndpointName, &quota.ClientType,
		&periodStartStr, &periodEndStr, &quota.TokensUsed,
		&quota.QuotaLimit, &lastUpdatedStr,
	)

	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	// 解析时间
	formats := []string{
		"2006-01-02 15:04:05.999999-07:00",
		"2006-01-02 15:04:05",
		"2006-01-02T15:04:05Z",
		time.RFC3339,
	}

	for _, format := range formats {
		if t, err := time.Parse(format, periodStartStr); err == nil {
			quota.PeriodStart = t
			break
		}
	}
	for _, format := range formats {
		if t, err := time.Parse(format, periodEndStr); err == nil {
			quota.PeriodEnd = t
			break
		}
	}
	for _, format := range formats {
		if t, err := time.Parse(format, lastUpdatedStr); err == nil {
			quota.LastUpdated = t
			break
		}
	}

	return &quota, nil
}

// UpdateEndpointQuota updates or creates a quota record
func (s *SQLiteStorage) UpdateEndpointQuota(quota *EndpointQuota) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	clientType := quota.ClientType
	if clientType == "" {
		clientType = "claude"
	}

	_, err := s.db.Exec(`
		INSERT INTO endpoint_quotas (endpoint_name, client_type, period_start, period_end, tokens_used, quota_limit, last_updated)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(endpoint_name, client_type, period_start) DO UPDATE SET
			tokens_used = excluded.tokens_used,
			quota_limit = excluded.quota_limit,
			last_updated = excluded.last_updated
	`, quota.EndpointName, clientType, quota.PeriodStart, quota.PeriodEnd, quota.TokensUsed, quota.QuotaLimit, time.Now())

	return err
}

// ResetExpiredQuotas resets quotas that have passed their period end
func (s *SQLiteStorage) ResetExpiredQuotas() error {
	s.mu.Lock()
	defer s.mu.Unlock()

	// 删除过期的配额记录（保留最新的一条用于历史参考）
	_, err := s.db.Exec(`
		DELETE FROM endpoint_quotas
		WHERE period_end < datetime('now')
		AND id NOT IN (
			SELECT MAX(id) FROM endpoint_quotas
			GROUP BY endpoint_name, client_type
		)
	`)

	return err
}

// GetRecentRequestsByEndpoint 查询指定端点最近N次请求记录
func (s *SQLiteStorage) GetRecentRequestsByEndpoint(endpointName string, clientType string, limit int) ([]RequestStat, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// Default client_type to 'claude' if not specified
	if clientType == "" {
		clientType = "claude"
	}

	query := `
		SELECT id, endpoint_name, COALESCE(client_type, 'claude') as client_type, COALESCE(client_ip, '') as client_ip,
			request_id, timestamp, date,
			input_tokens, cache_creation_tokens, cache_read_tokens, output_tokens,
			model, is_streaming, success, device_id, COALESCE(duration_ms, 0) as duration_ms,
			COALESCE(error_message, '') as error_message
		FROM request_stats
		WHERE endpoint_name=? AND COALESCE(client_type, 'claude')=?
		ORDER BY timestamp DESC
		LIMIT ?
	`

	rows, err := s.db.Query(query, endpointName, clientType, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []RequestStat
	for rows.Next() {
		var stat RequestStat
		if err := rows.Scan(
			&stat.ID, &stat.EndpointName, &stat.ClientType, &stat.ClientIP,
			&stat.RequestID, &stat.Timestamp, &stat.Date,
			&stat.InputTokens, &stat.CacheCreationTokens, &stat.CacheReadTokens, &stat.OutputTokens,
			&stat.Model, &stat.IsStreaming, &stat.Success, &stat.DeviceID, &stat.DurationMs,
			&stat.ErrorMessage,
		); err != nil {
			return nil, err
		}
		stats = append(stats, stat)
	}

	return stats, rows.Err()
}
