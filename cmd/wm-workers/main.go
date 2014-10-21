package main

import (
	"fmt"
	"os"
	"time"

	"github.com/codegangsta/cli"
)

var (
	VersionString   = "?"
	RevisionString  = "?"
	GeneratedString = "?"
)

func customVersionPrinter(c *cli.Context) {
	fmt.Printf("%v v=%v rev=%v d=%v\n", c.App.Name, VersionString, RevisionString, GeneratedString)
}

func main() {
	cli.VersionPrinter = customVersionPrinter

	app := cli.NewApp()
	app.Action = runWorkers
	app.Run(os.Args)
}

func runWorkers(c *cli.Context) {
	for {
		time.Sleep(1000 * time.Millisecond)
		fmt.Println("zzz")
	}
}
