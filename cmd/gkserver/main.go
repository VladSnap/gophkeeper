package main

import (
	"fmt"
	"os"

	"github.com/VladSnap/gophkeeper/internal/server/app"
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

	log.Zap.Info("Starting gophkeeper server application")

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

	// Run application
	if err := app.Run(); err != nil {
		log.Zap.Error("app run", zap.Error(err))
		os.Exit(2)
	}

	log.Zap.Info("Application completed successfully")
}
