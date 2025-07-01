package main

import (
	"fmt"
	"os"

	"github.com/VladSnap/gophkeeper/internal/client/app"
	"github.com/VladSnap/gophkeeper/internal/client/ui"
	"github.com/VladSnap/gophkeeper/pkg/log"
	"go.uber.org/zap"
)

func main() {
	// Initialize logger
	defer func() {
		if err := log.Close(); err != nil {
			fmt.Printf("Failed to close logger: %v\n", err)
		}
	}()

	log.Zap.Info("Starting gophkeeper client application")

	app := app.New()
	if err := app.Init(); err != nil {
		log.Zap.Error("app init", zap.Error(err))
		os.Exit(1)
	}
	// Register cleanup app function
	defer func() {
		if err := app.Stop(); err != nil {
			log.Zap.Error("app stop", zap.Error(err))
		}
	}()

	cli := ui.NewCLI(app)
	// Run CLI
	if err := cli.Init(); err != nil {
		log.Zap.Error("CLI init", zap.Error(err))
		os.Exit(2)
	}
	// Run CLI
	if err := cli.Run(); err != nil {
		log.Zap.Error("CLI run", zap.Error(err))
		os.Exit(3)
	}

	log.Zap.Info("Application completed successfully")
}
