package inits

import (
	"github.com/mszlu521/thunder/database"
	"github.com/mszlu521/thunder/logs"
)

func ensurePostgresSchema() {
	db := database.GetPostgresDB().GormDB
	statements := []string{
		`ALTER TABLE agents ADD COLUMN IF NOT EXISTS agent_mode varchar(50) NOT NULL DEFAULT 'general'`,
		`ALTER TABLE agents ADD COLUMN IF NOT EXISTS deep_config jsonb`,
		`CREATE TABLE IF NOT EXISTS skills (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL,
			deleted_at timestamptz,
			name varchar(100) NOT NULL UNIQUE,
			description text,
			base_dir varchar(512) NOT NULL,
			source_id varchar(255) NOT NULL,
			status varchar(20) NOT NULL DEFAULT 'active',
			creator_id uuid NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_skills_deleted_at ON skills(deleted_at)`,
		`CREATE TABLE IF NOT EXISTS github_sources (
			id uuid PRIMARY KEY DEFAULT gen_random_uuid(),
			created_at timestamptz NOT NULL,
			updated_at timestamptz NOT NULL,
			deleted_at timestamptz,
			name varchar(100) NOT NULL UNIQUE,
			repo_url varchar(512) NOT NULL,
			description text,
			creator_id uuid NOT NULL
		)`,
		`CREATE INDEX IF NOT EXISTS idx_github_sources_deleted_at ON github_sources(deleted_at)`,
		`CREATE TABLE IF NOT EXISTS agent_skills (
			agent_id uuid NOT NULL,
			skill_id uuid NOT NULL,
			status varchar(20) NOT NULL DEFAULT 'active',
			created_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
			updated_at timestamptz NOT NULL DEFAULT CURRENT_TIMESTAMP,
			PRIMARY KEY(agent_id, skill_id)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_agent_skill ON agent_skills(agent_id)`,
		`CREATE INDEX IF NOT EXISTS idx_skill_agent ON agent_skills(skill_id)`,
	}

	for _, statement := range statements {
		if err := db.Exec(statement).Error; err != nil {
			logs.Errorf("ensure postgres schema error: %v", err)
			panic(err)
		}
	}
}
