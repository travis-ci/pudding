package workers

import (
	"time"

	"github.com/Sirupsen/logrus"
)

type miniWorkers struct {
	cfg *config
	log *logrus.Logger
	r   *MiddlewareRaven
	w   map[string]func() error
}

func newMiniWorkers(cfg *config, log *logrus.Logger, r *MiddlewareRaven) *miniWorkers {
	return &miniWorkers{
		cfg: cfg,
		log: log,
		r:   r,
		w:   map[string]func() error{},
	}
}

func (mw *miniWorkers) Register(name string, f func() error) {
	mw.w[name] = f
}

func (mw *miniWorkers) Run() {
	mw.log.Debug("entering mini worker run loop")
	for {
		mw.runTick()
	}
}

func (mw *miniWorkers) runTick() {
	defer func() {
		if err := recover(); err != nil {
			mw.log.WithField("err", err).Error("recovered from panic")
		}
	}()

	for name, f := range mw.w {
		mw.log.WithField("job", name).Debug("running mini worker job")

		err := mw.r.Do(f)
		if err != nil {
			mw.log.WithFields(logrus.Fields{
				"err": err,
				"job": name,
			}).Error("mini worker job failed")
		}
	}

	mw.log.WithField("seconds", mw.cfg.MiniWorkerInterval).Info("mini workers sleeping")
	time.Sleep(time.Duration(int32(mw.cfg.MiniWorkerInterval)) * time.Second)
}
