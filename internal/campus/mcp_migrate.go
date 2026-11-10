package campus

import (
	"context"
	"database/sql"
	_ "embed"
)

//go:embed mcp_schema.sql
var mcpSchema string

func migrateMCP(ctx context.Context, db *sql.DB) error {
	if err := runAISchema(ctx, db, mcpSchema); err != nil {
		return err
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, "INSERT IGNORE INTO ai_migrations(name) VALUES('lesson06-legacy-tool')")
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 0 {
		_, err = tx.ExecContext(ctx, `INSERT INTO skill_tool_refs(skill_id,class_id,ref,tool_name)
   SELECT id,class_id,'knowledge_search','knowledge_search' FROM skills WHERE declared_tool='knowledge_search'`)
		if err != nil {
			return err
		}
	}
	return tx.Commit()
}
