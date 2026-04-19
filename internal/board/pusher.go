package board

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"github.com/jam941/Vestaboard-Golang/vestaboard"
)

const minInterval = 15 * time.Second

// Pusher is a rate-limited wrapper around the Vestaboard client.
type Pusher struct {
	client   *vestaboard.Client
	lastPush time.Time
	pending chan vestaboard.BoardLayout
	done    chan struct{}
}

func NewPusher(client *vestaboard.Client) *Pusher {
	p := &Pusher{
		client:  client,
		pending: make(chan vestaboard.BoardLayout, 1),
		done:    make(chan struct{}),
	}
	go p.loop()
	return p
}


func (p *Pusher) Push(ctx context.Context, layout vestaboard.BoardLayout) error {
	select {
	case <-p.pending:
	default:
	}

	select {
	case p.pending <- layout:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (p *Pusher) Stop() {
	close(p.pending)
	<-p.done
}

func (p *Pusher) loop() {
	defer close(p.done)

	for layout := range p.pending {
		// Enforce rate limit.
		elapsed := time.Since(p.lastPush)
		if elapsed < minInterval {
			sleep := minInterval - elapsed
			slog.Info("rate limit: sleeping before push", "sleep", sleep.Round(time.Millisecond))
			time.Sleep(sleep)
		}

		slog.Info("pushing layout to board")
		_, err := p.client.SendCharacters(layout, false)
		if err != nil {
			slog.Error("failed to push layout", "error", err)
		} else {
			slog.Info("board updated successfully")
		}
		p.lastPush = time.Now()
	}
}

var ErrPusherStopped = fmt.Errorf("pusher stopped")
