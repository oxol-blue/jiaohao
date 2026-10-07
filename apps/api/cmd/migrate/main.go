package main

import (
	"fmt"
	"os"

	"jiaohao/internal/config"
	"jiaohao/internal/db"
	"jiaohao/internal/db/migrations"
)

func main() {
	direction := "up"
	if len(os.Args) > 1 {
		direction = os.Args[1]
		if direction == "force" && len(os.Args) > 2 {
			direction = "force " + os.Args[2]
		}
	}
	cfg, err := config.Load()
	if err != nil {
		fmt.Fprintf(os.Stderr, "config: %v\n", err)
		os.Exit(1)
	}
	if err := db.Run(cfg.DatabaseURL, cfg.DatabaseSchema, migrations.Files, direction); err != nil {
		fmt.Fprintf(os.Stderr, "migrate %s: %v\n", direction, err)
		os.Exit(1)
	}
	fmt.Printf("migrate %s: ok\n", direction)
}
