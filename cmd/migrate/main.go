package main

import (
	"fmt"
	"os"

	"virungapay-be/internal/config"
	"virungapay-be/internal/database"
)

func main() {
	if len(os.Args) != 2 || (os.Args[1] != "up" && os.Args[1] != "down") {
		fmt.Fprintln(os.Stderr, "usage: migrate up|down")
		os.Exit(2)
	}

	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}

	if os.Args[1] == "up" {
		err = database.MigrateUp(cfg.Database.URL, "migrations")
	} else {
		err = database.MigrateDown(cfg.Database.URL, "migrations")
	}
	if err != nil {
		fmt.Fprintf(os.Stderr, "migrate: %v\n", err)
		os.Exit(1)
	}
}
