// knowtrade package is designed to configure and run your trade strategies,
// without having to do routine operations
package knowtrade

import (
	"time"

	ctx "github.com/tgmk/know-trade/context"

	"github.com/sirupsen/logrus"

	"github.com/tgmk/know-trade/config"
	"github.com/tgmk/know-trade/data"
)

// Handler for implements your trade logic
type Handler func(ctx *ctx.Context, settings *RunSettings) error

// ErrHandler handles your trade strategies logic errors
type ErrHandler func(ctx *ctx.Context, err error) error

// strategy represents strategy runner
type strategy struct {
	ctx       *ctx.Context
	isStopped bool

	errCh chan error
	errH  ErrHandler

	howRun HowRun

	log logrus.FieldLogger
}

func New(
	ctx *ctx.Context,
	howRun HowRun,
	errH ErrHandler,
) *strategy {
	return &strategy{
		ctx: ctx,

		errCh: make(chan error),
		errH:  errH,

		howRun: howRun,

		log: logrus.New(),
	}
}

func (s *strategy) GetData() *data.Data {
	return s.ctx.GetData()
}

// Run runs your trade strategy logic
func (s *strategy) Run(errH ErrHandler) {
	go s.ctx.GetData().Process()

	if errH != nil {
		s.errH = errH
		go s.processErrors(s.errH)
	}

	for runType, h := range s.howRun {
		runType := runType
		h := h
		switch runType {
		case config.TickerRun:
			go s.tickerRun(h)
		case config.EveryCandleRun:
			go s.byCandleRun(h)
		case config.EveryMatchRun:
			go s.byMatchRun(h)
		case config.EveryPositionChangeRun:
			go s.byPositionRun(h)
		//case config.ByOthersRun:
		default:
			panic("unknown run type")
		}
	}
}

func (s *strategy) Stop() {
	if s.isStopped {
		return
	}

	s.ctx.CancelFunc()
	s.isStopped = true
}

func (s *strategy) tickerRun(settings *RunSettings) {
	ticker := time.NewTicker(settings.TickerInterval)
	defer ticker.Stop()

	for {
		select {
		case <-s.ctx.Done():
			return
		case <-ticker.C:
			err := settings.Handler(s.ctx, settings)
			if err != nil {
				s.log.WithError(err).WithField("run", "ticker").Error("strategy execute failed")
				if s.errH != nil {
					s.errCh <- err
				}
			}
		}
	}
}

func (s *strategy) byCandleRun(settings *RunSettings) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.GetData().CandleCh():
			err := settings.Handler(s.ctx, settings)
			if err != nil {
				s.log.WithError(err).WithField("run", "every_candle").Error("strategy execute failed")
				if s.errH != nil {
					s.errCh <- err
				}
			}
		}
	}
}

func (s *strategy) byMatchRun(settings *RunSettings) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.GetData().MatchCh():
			err := settings.Handler(s.ctx, settings)
			if err != nil {
				s.log.WithError(err).WithField("run", "every_match").Error("strategy execute failed")
				if s.errH != nil {
					s.errCh <- err
				}
			}
		}
	}
}

func (s *strategy) byPositionRun(settings *RunSettings) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case <-s.GetData().PositionCh():
			err := settings.Handler(s.ctx, settings)
			if err != nil {
				s.log.WithError(err).WithField("run", "every_position_change").
					Error("strategy execute failed")
				if s.errH != nil {
					s.errCh <- err
				}
			}
		}
	}
}

func (s *strategy) byOthersRun(settings *RunSettings) {
	err := settings.Handler(s.ctx, settings)
	if err != nil {
		s.log.WithError(err).WithField("run", "other").Error("strategy execute failed")
		if s.errH != nil {
			s.errCh <- err
		}
	}
}

func (s *strategy) processErrors(errH ErrHandler) {
	for {
		select {
		case <-s.ctx.Done():
			return
		case err := <-s.errCh:
			if err != nil {
				resultErr := errH(s.ctx, err)
				if resultErr != nil {
					s.log.WithError(err).Error("exit by error process")
					return
				}
			}
		}
	}
}
