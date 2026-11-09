package connectip

import (
	"context"
	"sync"
)

type contextDatagramReader interface {
	ReceiveDatagram(context.Context) ([]byte, error)
}

// datagramReceiver makes an existing bounded QUIC queue selectable alongside
// stream capsules. Its unbuffered handoff retains at most one additional packet.
type datagramReceiver struct {
	reader  contextDatagramReader
	ctx     context.Context
	started sync.Once
	packets chan []byte
	done    chan struct{}
	cancel  context.CancelFunc
	err     error // published by closing done
}

func newDatagramReceiver(reader contextDatagramReader) *datagramReceiver {
	ctx, cancel := context.WithCancel(context.Background())
	r := &datagramReceiver{
		reader:  reader,
		ctx:     ctx,
		packets: make(chan []byte),
		done:    make(chan struct{}),
		cancel:  cancel,
	}
	return r
}

func (r *datagramReceiver) start() {
	r.started.Do(func() {
		go r.run(r.ctx, r.reader)
	})
}

func (r *datagramReceiver) run(ctx context.Context, reader contextDatagramReader) {
	defer close(r.done)
	for {
		packet, err := reader.ReceiveDatagram(ctx)
		if err != nil {
			r.err = err
			return
		}
		select {
		case r.packets <- packet:
		case <-ctx.Done():
			r.err = ctx.Err()
			return
		}
	}
}

func (r *datagramReceiver) stop() {
	if r != nil {
		r.cancel()
		r.started.Do(func() {
			r.err = r.ctx.Err()
			close(r.done)
		})
		<-r.done
	}
}
