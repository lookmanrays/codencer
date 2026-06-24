package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestDaemonEntrypointDoesNotLoadRepoDotEnv(t *testing.T) {
	repoRoot := filepath.Join("..", "..")
	for _, rel := range []string{
		filepath.Join("cmd", "orchestratord", "main.go"),
		filepath.Join("internal", "app", "bootstrap.go"),
	} {
		data, err := os.ReadFile(filepath.Join(repoRoot, rel))
		if err != nil {
			t.Fatal(err)
		}
		source := string(data)
		for _, forbidden := range []string{
			"github.com/joho/godotenv",
			"godotenv.Load",
		} {
			if strings.Contains(source, forbidden) {
				t.Fatalf("%s must not implicitly load repo .env; found %q", rel, forbidden)
			}
		}
	}
}
