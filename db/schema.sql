CREATE TABLE IF NOT EXISTS users (
    id SERIAL PRIMARY KEY,
    email VARCHAR(255) UNIQUE NOT NULL,
    password_hash VARCHAR(255) NOT NULL,
    rate_limit_minute INTEGER DEFAULT 0,  -- 0 = use server default
    rate_limit_daily INTEGER DEFAULT 0,   -- 0 = use server default
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS api_keys (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    name VARCHAR(255) NOT NULL,
    key_hash VARCHAR(255) UNIQUE NOT NULL,
    prefix VARCHAR(10) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS provider_keys (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    provider VARCHAR(50) NOT NULL, -- 'openai', 'anthropic'
    encrypted_key TEXT NOT NULL,
    label VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS model_aliases (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    alias VARCHAR(255) NOT NULL, -- e.g. 'prod-gpt4'
    target_model VARCHAR(255) NOT NULL, -- e.g. 'gpt-4' (upstream model id)
    provider_key_id INTEGER REFERENCES provider_keys(id), -- Primary provider
    fallback_alias_id INTEGER REFERENCES model_aliases(id), -- New fallback to another alias
    use_light_model BOOLEAN DEFAULT FALSE,
    light_model_threshold INTEGER DEFAULT 100, -- Number of tokens that when we're under we fallback to smaller model
    light_model VARCHAR(255),
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(user_id, alias)
);

CREATE TABLE IF NOT EXISTS provider_models (
    id SERIAL PRIMARY KEY,
    provider VARCHAR(50) NOT NULL, -- e.g. 'openai', 'anthropic'
    model_id VARCHAR(255) NOT NULL,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP,
    UNIQUE(provider, model_id)
);

CREATE TABLE IF NOT EXISTS request_logs (
    id SERIAL PRIMARY KEY,
    user_id INTEGER REFERENCES users(id),
    alias_used VARCHAR(255),
    provider_used VARCHAR(50),
    model_used VARCHAR(255),
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    status_code INTEGER,
    created_at TIMESTAMP WITH TIME ZONE DEFAULT CURRENT_TIMESTAMP
);
