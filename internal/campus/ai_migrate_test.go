package campus

import (
	"context"
	"testing"
)

func TestAIMigrationPreservesData(t *testing.T) {
	s, _ := testServer(t)
	ctx := context.Background()
	exec := func(query string, args ...any) {
		t.Helper()
		if _, err := s.DB.ExecContext(ctx, query, args...); err != nil {
			t.Fatal(err)
		}
	}
	count := func(want int, query string) {
		t.Helper()
		var got int
		if err := s.DB.QueryRowContext(ctx, query).Scan(&got); err != nil || got != want {
			t.Fatalf("got %d, want %d: %v", got, want, err)
		}
	}
	count(4, "SELECT COUNT(*) FROM assistants WHERE enabled")
	count(2, "SELECT COUNT(*) FROM assistant_skills")
	exec("INSERT INTO assistants(class_id,name) SELECT id,'old assistant' FROM classes WHERE code='A'")
	exec("INSERT INTO skills(class_id,assistant_id,name,description) SELECT class_id,id,'old skill','legacy text' FROM assistants WHERE name='old assistant'")
	// Simulate a database upgraded from the previous version, with no backfill marker.
	exec("DELETE FROM ai_migrations WHERE name='lesson05-legacy-skill-links'")
	if err := Migrate(ctx, s.DB); err != nil {
		t.Fatal(err)
	}
	count(1, "SELECT COUNT(*) FROM assistant_skills b JOIN skills k ON k.id=b.skill_id WHERE k.name='old skill'")
	exec("UPDATE assistants SET name='edited',enabled=FALSE WHERE builtin_key='problem-guide'")
	exec("DELETE FROM assistant_skills WHERE skill_id IN (SELECT id FROM skills WHERE builtin_key='problem-guide' OR name='old skill')")
	if err := Migrate(ctx, s.DB); err != nil {
		t.Fatal(err)
	}
	count(2, "SELECT COUNT(*) FROM assistants WHERE name='edited' AND NOT enabled")
	count(1, "SELECT COUNT(*) FROM skills WHERE name='old skill' AND description='legacy text' AND NOT enabled")
	count(0, "SELECT COUNT(*) FROM assistant_skills")
	count(4, "SELECT COUNT(*) FROM users")
	count(2, "SELECT COUNT(*) FROM knowledge_entries")
	if _, err := s.DB.ExecContext(ctx, `INSERT INTO assistant_skills(assistant_id,skill_id,class_id)
 SELECT a.id,k.id,a.class_id FROM assistants a JOIN skills k ON k.class_id<>a.class_id
 WHERE a.builtin_key='course-qa' LIMIT 1`); err == nil {
		t.Fatal("cross-class binding succeeded")
	}
}
