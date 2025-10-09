package main

import (
	"log"
	"os"

	"github.com/Servflow/servflow/cmd"
)

func main() {
	app := cmd.CreateApp()

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}
