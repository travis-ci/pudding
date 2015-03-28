package pudding

import (
	"fmt"
	"os"
	"time"

	"github.com/codegangsta/cli"
)

const (
	generatedTimeFormat = "2006-01-02T15:04:05-0700"
)

var (
	// VersionString is the git describe version set at build time
	VersionString = "?"
	// RevisionString is the git revision set at build time
	RevisionString = "?"
	// GeneratedString is the build date set at build time
	GeneratedString = "?"
)

func init() {
	cli.VersionPrinter = customVersionPrinter
	_ = os.Setenv("VERSION", VersionString)
	_ = os.Setenv("REVISION", RevisionString)
	_ = os.Setenv("GENERATED", GeneratedString)
}

func customVersionPrinter(c *cli.Context) {
	fmt.Printf("%v v=%v rev=%v d=%v\n",
		c.App.Name, c.App.Version, RevisionString, c.App.Compiled.Format(generatedTimeFormat))
}

// GeneratedTime returns the parsed GeneratedString if it isn't `?`
func GeneratedTime() time.Time {
	if GeneratedString != "?" {
		t, err := time.Parse(generatedTimeFormat, GeneratedString)
		if err == nil {
			return t
		}
	}

	info, err := os.Stat(os.Args[0])
	if err != nil {
		return time.Now().UTC()
	}
	return info.ModTime()
}
