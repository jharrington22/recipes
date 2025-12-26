package migrate

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

// ApplyDirectory executes all *.sql files in dir in lexical order.
// This is intentionally simple (good for local/dev).
func ApplyDirectory(ctx context.Context, pool *pgxpool.Pool, dir string) error {
	ents, err := os.ReadDir(dir)
	if err != nil {
		return err
	}

	files := make([]string, 0)
	for _, e := range ents {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if strings.HasSuffix(strings.ToLower(name), ".sql") {
			files = append(files, filepath.Join(dir, name))
		}
	}
	sort.Strings(files)

	for _, f := range files {
		b, err := os.ReadFile(f)
		if err != nil {
			return err
		}
		sql := string(b)
		if strings.TrimSpace(sql) == "" {
			continue
		}

		if _, err := pool.Exec(ctx, sql); err != nil {
			return fmt.Errorf("migration %s failed: %w", filepath.Base(f), err)
		}
	}
	return nil
}
