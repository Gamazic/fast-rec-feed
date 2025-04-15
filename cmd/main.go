package main

import (
	"fastapp/internal/api"
	"log"
)

func main() {
	app := api.NewApp()
	log.Fatal(app.Run())
}
