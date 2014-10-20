package main

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/codegangsta/cli"
)

func main() {
	log := logrus.New()

	app := cli.NewApp()
	app.Action = func(_ *cli.Context) {
		log.Info("Not much here")
	}

	app.Run(os.Args)
}
