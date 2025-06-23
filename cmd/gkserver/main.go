package main

import (
	"github.com/VladSnap/gophkeeper/internal/server/app"
	"github.com/VladSnap/gophkeeper/internal/server/storage"
)

func main() {
	conf, _ := app.ParseFlags()
	db, _ := storage.NewDatabaseServer(conf.DatabaseURI)
	db.InitDatabase()
	db.Close()
}
