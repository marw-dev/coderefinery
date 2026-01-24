-- Erweitere repositories Tabelle (ehemals projects)
ALTER TABLE repositories
ADD COLUMN IF NOT EXISTS index_config JSONB NOT NULL DEFAULT '{}',
ADD COLUMN IF NOT EXISTS default_pipeline JSONB NOT NULL DEFAULT '{}',
ADD COLUMN IF NOT EXISTS total_executions INTEGER NOT NULL DEFAULT 0,
ADD COLUMN IF NOT EXISTS last_executed_at TIMESTAMP;

-- Executions Tabelle für Analytics & History
CREATE TABLE IF NOT EXISTS pipeline_executions (
    id UUID PRIMARY KEY,
    repository_id UUID NOT NULL REFERENCES repositories(id) ON DELETE CASCADE,

    mode VARCHAR(20) NOT NULL, -- 'fast_path', 'hybrid', 'full'
    complexity VARCHAR(20) NOT NULL, -- 'simple', 'medium', 'complex'

    query TEXT NOT NULL,
    final_output TEXT,

    stages JSONB NOT NULL DEFAULT '[]', -- Speichert die Details jeder Stage

    total_duration_ms INTEGER NOT NULL,
    success BOOLEAN NOT NULL DEFAULT false,

    created_at TIMESTAMP NOT NULL DEFAULT NOW()
);

CREATE INDEX idx_executions_repo ON pipeline_executions(repository_id);
CREATE INDEX idx_executions_created ON pipeline_executions(created_at DESC);
