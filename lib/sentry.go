package lib

import (
	"github.com/Sirupsen/logrus"
	"github.com/getsentry/raven-go"
)

// SendRavenPacket encapsulates the raven packet send, plus logging
// around errors and such
func SendRavenPacket(packet *raven.Packet, cl *raven.Client, log *logrus.Logger) error {
	log.WithFields(logrus.Fields{
		"packet": packet,
	}).Info("sending sentry packet")

	eventID, ch := cl.Capture(packet, map[string]string{})
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
