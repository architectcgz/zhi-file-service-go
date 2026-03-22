package main

import (
	"flag"
	"fmt"
	"os"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/migrations"
)

func main() {
	source := flag.String("source", "migrations", "source migrations root directory")
	output := flag.String("output", ".build/migrations/all", "flattened output directory for golang-migrate")
	flag.Parse()

	collected, err := migrations.BuildLinearView(*source, *output)
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate-runner failed: %v\n", err)
		os.Exit(1)
	}

	fmt.Printf("migrate-runner: generated %d migration versions into %s\n", len(collected), *output)
	for _, migration := range collected {
		fmt.Printf("  - %06d_%s (%s)\n", migration.Version, migration.Name, migration.Schema)
	}
}
