package database

import "testing"

func TestToMigrateURL(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"postgres://u:p@h:5432/db?sslmode=disable":              "pgx5://u:p@h:5432/db?sslmode=disable",
		"postgresql://u:p@h/db":                                 "pgx5://u:p@h/db",
		"postgres://u:p@h/db?pool_max_conns=10&sslmode=disable": "pgx5://u:p@h/db?sslmode=disable",
		"postgres://u:p@h/db?pool_max_conns=10":                 "pgx5://u:p@h/db",
	}
	for in, want := range tests {
		if got := toMigrateURL(in); got != want {
			t.Errorf("toMigrateURL(%q) = %q, want %q", in, got, want)
		}
	}
}
