package lib

import (
	"os"

	"github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
	"github.com/travis-ci/worker/lib"
)

var (
	// SentryTags are the tags provided to each sentry client and are applied to
	// each packet sent to sentry
	SentryTags = map[string]string{
		"level":    "panic",
		"logger":   "root",
		"dyno":     os.Getenv("DYNO"),
		"hostname": os.Getenv("HOSTNAME"),
		"revision": lib.RevisionString,
		"version":  lib.VersionString,
	}
)

// SendRavenPacket encapsulates the raven packet send, plus logging
// around errors and such
func SendRavenPacket(packet *raven.Packet, cl *raven.Client, log *logrus.Logger, tags map[string]string) error {
	log.WithFields(logrus.Fields{
		"packet": packet,
	}).Info("sending sentry packet")

	eventID, ch := cl.Capture(packet, tags)
	err := <-ch
	if err != nil {
		log.WithFields(logrus.Fields{
			"event_id": eventID,
			"err":      err,
		}).Error("problem sending sentry packet")
	} else {
		log.WithFields(logrus.Fields{
			"event_id": eventID,
		}).Info("successfully sent sentry packet")
	}

	return err
}
