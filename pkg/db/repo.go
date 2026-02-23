package db

import (
	"context"
	"fmt"
	"time"
)

// ModelAlias represents a routing rule in the database
type ModelAlias struct {
	ID                  int
	UserID              int
	Alias               string
	TargetModel         string
	ProviderKeyID       int
	FallbackAliasID     *int
	UseLightModel       bool
	LightModelThreshold int
	LightModel          *string
}

// ProviderKey represents a downstream provider's key
type ProviderKey struct {
	ID           int
	UserID       int
	Provider     string
	EncryptedKey string
	Label        string
	CreatedAt    time.Time
}

// RequestLog represents a logged request
type RequestLog struct {
	UserID       int
	AliasUsed    string
	ProviderUsed string
	ModelUsed    string
	InputTokens  int
	OutputTokens int
	StatusCode   int
}

// UsageStats represents aggregated usage data
type UsageStats struct {
	Provider string
	Alias    string
	Input    int
	Output   int
	Reqs     int
}

// Repository defines the interface for all database operations
type Repository interface {
	// Auth & Users
	CreateUser(ctx context.Context, email, passwordHash string) (int, error)
	GetUserByEmail(ctx context.Context, email string) (int, string, error)
	GetUserByID(ctx context.Context, userID int) (email string, rateLimitMinute, rateLimitDaily int, err error)

	// API Keys
	CreateAPIKey(ctx context.Context, userID int, name, keyHash, prefix string) error

	// Model Aliases
	UpsertModelAlias(ctx context.Context, userID int, alias, targetModel string, providerKeyID int, fallbackAliasID *int, useLightModel bool, lightModelThreshold int, lightModel *string) error
	GetModelAlias(ctx context.Context, userID int, alias string) (*ModelAlias, error)
	GetModelAliasByID(ctx context.Context, id int) (string, error)
	ListModelAliases(ctx context.Context, userID int) ([]ModelAlias, error)
	PatchModelAlias(ctx context.Context, userID int, alias string, updates map[string]interface{}) error

	// Provider Keys
	CreateProviderKey(ctx context.Context, userID int, provider, encryptedKey, label string) error
	GetProviderKey(ctx context.Context, keyID int, userID int) (string, string, error)
	ListProviderKeys(ctx context.Context, userID int) ([]ProviderKey, error)
	ListUniqueProviderKeysPerProvider(ctx context.Context) ([]ProviderKey, error)

	// Provider Models
	InsertProviderModel(ctx context.Context, provider, modelID string) error
	ListProviderModelsByType(ctx context.Context, providerType string) ([]string, error)
	ListAllProviderModels(ctx context.Context) (map[string][]string, error)

	// Request Logs
	InsertRequestLog(ctx context.Context, log RequestLog) error
	GetUsageStats(ctx context.Context, userID int) ([]UsageStats, error)
}

type PostgresRepository struct {
	pool DB
}

func NewPostgresRepository(pool DB) *PostgresRepository {
	return &PostgresRepository{pool: pool}
}

func (r *PostgresRepository) CreateUser(ctx context.Context, email, passwordHash string) (int, error) {
	var id int
	err := r.pool.QueryRow(ctx, "INSERT INTO users (email, password_hash) VALUES ($1, $2) RETURNING id", email, passwordHash).Scan(&id)
	return id, err
}

func (r *PostgresRepository) GetUserByEmail(ctx context.Context, email string) (int, string, error) {
	var id int
	var storedHash string
	err := r.pool.QueryRow(ctx, "SELECT id, password_hash FROM users WHERE email = $1", email).Scan(&id, &storedHash)
	return id, storedHash, err
}

func (r *PostgresRepository) GetUserByID(ctx context.Context, userID int) (string, int, int, error) {
	var email string
	var rateLimitMinute, rateLimitDaily int
	err := r.pool.QueryRow(ctx, "SELECT email, rate_limit_minute, rate_limit_daily FROM users WHERE id = $1", userID).Scan(&email, &rateLimitMinute, &rateLimitDaily)
	return email, rateLimitMinute, rateLimitDaily, err
}

func (r *PostgresRepository) CreateAPIKey(ctx context.Context, userID int, name, keyHash, prefix string) error {
	_, err := r.pool.Exec(ctx, "INSERT INTO api_keys (user_id, name, key_hash, prefix) VALUES ($1, $2, $3, $4)", userID, name, keyHash, prefix)
	return err
}

func (r *PostgresRepository) UpsertModelAlias(ctx context.Context, userID int, alias, targetModel string, providerKeyID int, fallbackAliasID *int, useLightModel bool, lightModelThreshold int, lightModel *string) error {
	sql := `INSERT INTO model_aliases (user_id, alias, target_model, provider_key_id, fallback_alias_id, use_light_model, light_model_threshold, light_model)
	        VALUES ($1, $2, $3, $4, $5, $6, $7, $8)
			ON CONFLICT (user_id, alias)
			DO UPDATE SET target_model = EXCLUDED.target_model,
			              provider_key_id = EXCLUDED.provider_key_id,
						  fallback_alias_id = EXCLUDED.fallback_alias_id,
						  use_light_model = EXCLUDED.use_light_model,
						  light_model_threshold = EXCLUDED.light_model_threshold,
						  light_model = EXCLUDED.light_model`
	_, err := r.pool.Exec(ctx, sql, userID, alias, targetModel, providerKeyID, fallbackAliasID, useLightModel, lightModelThreshold, lightModel)
	return err
}

func (r *PostgresRepository) GetModelAlias(ctx context.Context, userID int, alias string) (*ModelAlias, error) {
	var a ModelAlias
	err := r.pool.QueryRow(ctx,
		"SELECT target_model, provider_key_id, fallback_alias_id, use_light_model, light_model_threshold, light_model FROM model_aliases WHERE user_id = $1 AND alias = $2",
		userID, alias).Scan(&a.TargetModel, &a.ProviderKeyID, &a.FallbackAliasID, &a.UseLightModel, &a.LightModelThreshold, &a.LightModel)
	if err != nil {
		return nil, err
	}
	a.UserID = userID
	a.Alias = alias
	return &a, nil
}

func (r *PostgresRepository) GetModelAliasByID(ctx context.Context, id int) (string, error) {
	var alias string
	err := r.pool.QueryRow(ctx, "SELECT alias FROM model_aliases WHERE id = $1", id).Scan(&alias)
	return alias, err
}

func (r *PostgresRepository) ListModelAliases(ctx context.Context, userID int) ([]ModelAlias, error) {
	rows, err := r.pool.Query(ctx, "SELECT id, alias, target_model, provider_key_id, fallback_alias_id, use_light_model, light_model_threshold, light_model FROM model_aliases WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var aliases []ModelAlias
	for rows.Next() {
		var a ModelAlias
		err := rows.Scan(&a.ID, &a.Alias, &a.TargetModel, &a.ProviderKeyID, &a.FallbackAliasID, &a.UseLightModel, &a.LightModelThreshold, &a.LightModel)
		if err != nil {
			return nil, err
		}
		a.UserID = userID
		aliases = append(aliases, a)
	}
	return aliases, nil
}

// allowedPatchColumns is the whitelist of columns that can be updated via PATCH.
var allowedPatchColumns = map[string]bool{
	"target_model":          true,
	"provider_key_id":       true,
	"fallback_alias_id":     true,
	"use_light_model":       true,
	"light_model_threshold": true,
	"light_model":           true,
}

func (r *PostgresRepository) PatchModelAlias(ctx context.Context, userID int, alias string, updates map[string]interface{}) error {
	sqlStr := "UPDATE model_aliases SET "
	args := []interface{}{userID, alias}
	argIdx := 3
	for k, v := range updates {
		if !allowedPatchColumns[k] {
			continue
		}
		sqlStr += fmt.Sprintf("%s = $%d, ", k, argIdx)
		args = append(args, v)
		argIdx++
	}
	if argIdx == 3 {
		return fmt.Errorf("no valid fields to update")
	}
	sqlStr = sqlStr[:len(sqlStr)-2] + " WHERE user_id = $1 AND alias = $2"

	_, err := r.pool.Exec(ctx, sqlStr, args...)
	return err
}

func (r *PostgresRepository) CreateProviderKey(ctx context.Context, userID int, provider, encryptedKey, label string) error {
	_, err := r.pool.Exec(ctx, "INSERT INTO provider_keys (user_id, provider, encrypted_key, label) VALUES ($1, $2, $3, $4)", userID, provider, encryptedKey, label)
	return err
}

func (r *PostgresRepository) GetProviderKey(ctx context.Context, keyID int, userID int) (string, string, error) {
	var providerType, encryptedKey string
	err := r.pool.QueryRow(ctx, "SELECT provider, encrypted_key FROM provider_keys WHERE id = $1 AND user_id = $2", keyID, userID).Scan(&providerType, &encryptedKey)
	return providerType, encryptedKey, err
}

func (r *PostgresRepository) ListProviderKeys(ctx context.Context, userID int) ([]ProviderKey, error) {
	rows, err := r.pool.Query(ctx, "SELECT id, provider, label, created_at FROM provider_keys WHERE user_id = $1", userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []ProviderKey
	for rows.Next() {
		var k ProviderKey
		err := rows.Scan(&k.ID, &k.Provider, &k.Label, &k.CreatedAt)
		if err != nil {
			return nil, err
		}
		k.UserID = userID
		keys = append(keys, k)
	}
	return keys, nil
}

func (r *PostgresRepository) ListUniqueProviderKeysPerProvider(ctx context.Context) ([]ProviderKey, error) {
	rows, err := r.pool.Query(ctx, "SELECT DISTINCT ON (provider) id, user_id, provider FROM provider_keys")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var keys []ProviderKey
	for rows.Next() {
		var k ProviderKey
		err := rows.Scan(&k.ID, &k.UserID, &k.Provider)
		if err != nil {
			return nil, err
		}
		keys = append(keys, k)
	}
	return keys, nil
}

func (r *PostgresRepository) InsertProviderModel(ctx context.Context, provider, modelID string) error {
	_, err := r.pool.Exec(ctx, "INSERT INTO provider_models (provider, model_id) VALUES ($1, $2) ON CONFLICT DO NOTHING", provider, modelID)
	return err
}

func (r *PostgresRepository) ListProviderModelsByType(ctx context.Context, providerType string) ([]string, error) {
	rows, err := r.pool.Query(ctx, "SELECT model_id FROM provider_models WHERE provider = $1", providerType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var models []string
	for rows.Next() {
		var m string
		if err := rows.Scan(&m); err != nil {
			return nil, err
		}
		models = append(models, m)
	}
	return models, nil
}

func (r *PostgresRepository) ListAllProviderModels(ctx context.Context) (map[string][]string, error) {
	rows, err := r.pool.Query(ctx, "SELECT provider, model_id FROM provider_models")
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	models := make(map[string][]string)
	for rows.Next() {
		var p, m string
		if err := rows.Scan(&p, &m); err != nil {
			return nil, err
		}
		models[p] = append(models[p], m)
	}
	return models, nil
}

func (r *PostgresRepository) InsertRequestLog(ctx context.Context, log RequestLog) error {
	_, err := r.pool.Exec(ctx,
		"INSERT INTO request_logs (user_id, alias_used, provider_used, model_used, input_tokens, output_tokens, status_code) VALUES ($1, $2, $3, $4, $5, $6, $7)",
		log.UserID, log.AliasUsed, log.ProviderUsed, log.ModelUsed, log.InputTokens, log.OutputTokens, log.StatusCode)
	return err
}

func (r *PostgresRepository) GetUsageStats(ctx context.Context, userID int) ([]UsageStats, error) {
	sql := `SELECT provider_used, alias_used, SUM(input_tokens) as input, SUM(output_tokens) as output, COUNT(*) as reqs 
	        FROM request_logs 
			WHERE user_id = $1 
			GROUP BY provider_used, alias_used`

	rows, err := r.pool.Query(ctx, sql, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var stats []UsageStats
	for rows.Next() {
		var s UsageStats
		if err := rows.Scan(&s.Provider, &s.Alias, &s.Input, &s.Output, &s.Reqs); err != nil {
			return nil, err
		}
		stats = append(stats, s)
	}
	return stats, nil
}
