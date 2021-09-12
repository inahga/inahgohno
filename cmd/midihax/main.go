package main

import (
	"fmt"

	"gitlab.com/gomidi/midi"
	"gitlab.com/gomidi/midi/reader"
	"gitlab.com/gomidi/rtmididrv"
)

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
	rd := reader.New(
		reader.Each(func(pos *reader.Position, msg midi.Message) {
			fmt.Printf("got %s\n", msg)
		}),
	)

	if err := rd.ListenTo(in); err != nil {
		panic(err)
	}
	<-make(chan struct{})
}
