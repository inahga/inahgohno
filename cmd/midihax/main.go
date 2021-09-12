package main

import (
	"context"
	"fmt"
	"sync"
	"time"

	"gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/reader"
	"gitlab.com/gomidi/rtmididrv"
)

// Being tick based avoids the condition where user presses a bunch of keys at
// the same time, and the incoming MIDI messages coincidentally arrive in the
// correct order.
const tick = 200 * time.Millisecond

type State struct {
	state map[uint8]struct{}
	lock  sync.Mutex
}

func Start(ctx context.Context, in midi.In) error {
	state := NewState()
	ticker := time.NewTicker(tick)

	rd := reader.New(
		reader.NoteOn(func(p *reader.Position, channel, key, velocity uint8) {
			state.Set(key)
			// Reset the ticker so that there is a constant time between the user playing
			// the correct keys and the game recognizing it.
			ticker.Reset(tick)
		}),
		reader.NoteOff(func(p *reader.Position, channel, key, velocity uint8) {
			state.Unset(key)
			ticker.Reset(tick)
		}),
	)
	if err := rd.ListenTo(in); err != nil {
		return fmt.Errorf("failed to listen on MIDI device: %w", err)
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				if result := state.Check([]uint8{21, 23}); result {
					fmt.Println("yay")
				}
			}
		}
	}()

	<-ctx.Done()
	return ctx.Err()
}

func NewState() *State {
	return &State{
		state: map[uint8]struct{}{},
		lock:  sync.Mutex{},
	}
}

func (s *State) Set(key uint8) {
	s.lock.Lock()
	defer s.lock.Unlock()
	s.state[key] = struct{}{}
}

func (s *State) Unset(key uint8) {
	s.lock.Lock()
	defer s.lock.Unlock()
	delete(s.state, key)
}

func (s *State) Check(desired []uint8) bool {
	s.lock.Lock()
	defer s.lock.Unlock()
	if len(s.state) != len(desired) {
		return false
	}
	for _, key := range desired {
		if _, ok := s.state[key]; !ok {
			return false
		}
	}
	return true
}

func main() {
	const dev = 1

	drv, err := rtmididrv.New()
	if err != nil {
		panic(err)
	}
	defer drv.Close()

	ins, err := drv.Ins()
	if err != nil {
		panic(err)
	}
	in := ins[dev]

	fmt.Println(in)

	if err := in.Open(); err != nil {
		panic(err)
	}
	defer in.Close()

	if err := Start(context.Background(), in); err != nil {
		panic(err)
	}
}
