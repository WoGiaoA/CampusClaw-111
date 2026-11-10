package campus

import (
	"context"
	"database/sql"
	_ "embed"
	"strings"
)

//go:embed ai_schema.sql
var aiSchema string

type sqlExecutor interface {
	ExecContext(context.Context, string, ...any) (sql.Result, error)
}

func runAISchema(ctx context.Context, db *sql.DB, ddl string) error {
	for _, statement := range strings.Split(ddl, ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := db.ExecContext(ctx, statement); err != nil {
			return err
		}
	}
	return nil
}

func migrateAI(ctx context.Context, db *sql.DB) error {
	before, after, _ := strings.Cut(aiSchema, "-- AI COLUMN MIGRATIONS")
	if err := runAISchema(ctx, db, before); err != nil {
		return err
	}
	changes := []struct {
		table, catalog, field, name, ddl string
	}{
		{"assistants", "columns", "column_name", "enabled", "ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT FALSE"},
		{"assistants", "columns", "column_name", "deleted_at", "ADD COLUMN deleted_at DATETIME(6) NULL"},
		{"assistants", "columns", "column_name", "template_id", "ADD COLUMN template_id BIGINT NULL"},
		{"assistants", "columns", "column_name", "builtin_key", "ADD COLUMN builtin_key VARCHAR(40) NULL"},
		{"assistants", "statistics", "index_name", "assistant_builtin", "ADD UNIQUE KEY assistant_builtin(class_id,builtin_key)"},
		{"assistants", "table_constraints", "constraint_name", "assistant_template_class", "ADD CONSTRAINT assistant_template_class FOREIGN KEY(template_id,class_id) REFERENCES prompt_templates(id,class_id)"},
		{"skills", "columns", "column_name", "enabled", "ADD COLUMN enabled BOOLEAN NOT NULL DEFAULT FALSE"},
		{"skills", "columns", "column_name", "builtin_key", "ADD COLUMN builtin_key VARCHAR(40) NULL"},
		{"skills", "columns", "column_name", "conditions", "ADD COLUMN conditions TEXT NULL"},
		{"skills", "columns", "column_name", "procedure_text", "ADD COLUMN procedure_text TEXT NULL"},
		{"skills", "columns", "column_name", "raw_text", "ADD COLUMN raw_text TEXT NULL"},
		{"skills", "columns", "column_name", "material_terms", "ADD COLUMN material_terms TEXT NULL"},
		{"skills", "columns", "column_name", "declared_tool", "ADD COLUMN declared_tool VARCHAR(40) NULL"},
		{"skills", "statistics", "index_name", "skill_class", "ADD UNIQUE KEY skill_class(id,class_id)"},
		{"skills", "statistics", "index_name", "skill_builtin", "ADD UNIQUE KEY skill_builtin(class_id,builtin_key)"},
	}
	for _, change := range changes {
		var count int
		err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema."+change.catalog+" WHERE table_schema=DATABASE() AND table_name=? AND "+change.field+"=?", change.table, change.name).Scan(&count)
		if err != nil {
			return err
		}
		if count == 0 {
			if _, err := db.ExecContext(ctx, "ALTER TABLE "+change.table+" "+change.ddl); err != nil {
				return err
			}
		}
	}
	if err := runAISchema(ctx, db, after); err != nil {
		return err
	}
	for _, column := range []string{"raw_text", "procedure_text"} {
		var kind string
		if err := db.QueryRowContext(ctx, "SELECT data_type FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='skills' AND column_name=?", column).Scan(&kind); err != nil {
			return err
		}
		if kind == "text" {
			if _, err := db.ExecContext(ctx, "ALTER TABLE skills MODIFY COLUMN "+column+" MEDIUMTEXT NULL"); err != nil {
				return err
			}
		}
	}
	var deletedColumn int
	if err := db.QueryRowContext(ctx, "SELECT COUNT(*) FROM information_schema.columns WHERE table_schema=DATABASE() AND table_name='ai_conversations' AND column_name='deleted_at'").Scan(&deletedColumn); err != nil {
		return err
	}
	if deletedColumn == 0 {
		if _, err := db.ExecContext(ctx, "ALTER TABLE ai_conversations ADD COLUMN deleted_at DATETIME(6) NULL"); err != nil {
			return err
		}
	}
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	marker, err := tx.ExecContext(ctx, "INSERT IGNORE INTO ai_migrations(name) VALUES('lesson05-legacy-skill-links')")
	if err != nil {
		return err
	}
	inserted, err := marker.RowsAffected()
	if err != nil {
		return err
	}
	if inserted != 0 {
		_, err = tx.ExecContext(ctx, "INSERT INTO assistant_skills(assistant_id,skill_id,class_id) SELECT assistant_id,id,class_id FROM skills WHERE assistant_id IS NOT NULL")
		if err != nil {
			return err
		}
	}
	if err := seedAI(ctx, tx); err != nil {
		return err
	}
	return tx.Commit()
}

const guideConditions = "解题、怎么做、判别式"
const guideProcedure = "先询问题意和学生已经尝试的步骤，再循序给提示并请学生尝试下一步。默认不给最终答案、得数或完整解法。"

func seedAI(ctx context.Context, db sqlExecutor) error {
	for _, app := range []struct{ key, name, role, template string }{
		{"course-qa", "课程资料问答", "你是课程资料助教。优先依据相关资料回答，区分一般性知识。", "概括要点，说明依据和适用条件，使用清晰简洁的语言。"},
		{"problem-guide", "解题引导", "你是解题引导助手，帮助学生自行推导。", guideProcedure},
	} {
		_, err := db.ExecContext(ctx, `INSERT INTO prompt_templates(class_id,name,body,builtin_key)
    SELECT id,?,?,? FROM classes ON DUPLICATE KEY UPDATE id=prompt_templates.id`, app.name, app.template, app.key)
		if err != nil {
			return err
		}
		if app.key == "problem-guide" {
			_, err = db.ExecContext(ctx, `INSERT INTO skills(class_id,name,enabled,builtin_key,conditions,procedure_text,raw_text)
     SELECT id,'解题引导',TRUE,'problem-guide',?,?,? FROM classes ON DUPLICATE KEY UPDATE id=skills.id`,
				guideConditions, guideProcedure, "名称：解题引导\n适用条件："+guideConditions+"\n启用：是\n\n"+guideProcedure)
			if err != nil {
				return err
			}
			// Only bind for new applications. Restarting must preserve explicit unbinds.
			_, err = db.ExecContext(ctx, `INSERT INTO assistants(class_id,name,prompt_text,enabled,template_id,builtin_key)
     SELECT p.class_id,?,?,TRUE,p.id,p.builtin_key FROM prompt_templates p
     WHERE p.builtin_key=? AND NOT EXISTS (SELECT 1 FROM assistants a WHERE a.class_id=p.class_id AND a.builtin_key=p.builtin_key)`, app.name, app.role, app.key)
			if err != nil {
				return err
			}
			// The per-class marker is committed with the binding and never recreated.
			_, err = db.ExecContext(ctx, `INSERT INTO assistant_skills(assistant_id,skill_id,class_id)
     SELECT a.id,k.id,a.class_id FROM assistants a JOIN skills k ON k.class_id=a.class_id AND k.builtin_key='problem-guide'
     WHERE a.builtin_key='problem-guide' AND NOT EXISTS (SELECT 1 FROM ai_migrations m WHERE m.name=CONCAT('guide-seed-',a.class_id))`)
			if err != nil {
				return err
			}
			_, err = db.ExecContext(ctx, `INSERT IGNORE INTO ai_migrations(name) SELECT CONCAT('guide-seed-',class_id) FROM assistants WHERE builtin_key='problem-guide'`)
			if err != nil {
				return err
			}
			continue
		}
		_, err = db.ExecContext(ctx, `INSERT INTO assistants(class_id,name,prompt_text,enabled,template_id,builtin_key)
    SELECT class_id,?,?,TRUE,id,builtin_key FROM prompt_templates WHERE builtin_key=?
    ON DUPLICATE KEY UPDATE id=assistants.id`, app.name, app.role, app.key)
		if err != nil {
			return err
		}
	}
	return nil
}
