package main

import (
	"WB1/libr"
	"encoding/json"
	"fmt"
	"github.com/nats-io/stan.go"
	"math"
	"os"
	"os/signal"
	"time"
)

func main() {
	sc, err := stan.Connect("test-cluster", "client-publisher", stan.NatsURL("0.0.0.0:4222"))
	if err != nil {
		fmt.Println(err, 1)
		err = nil
	}

	for i := 0; i < math.MaxInt; i++ {
		s := libr.NewStrGen()
		JsonS, err := json.Marshal(s)
		if err != nil {
			fmt.Println(err, "JSON")
		}
		err = sc.Publish("foo", JsonS)
		if err != nil {
			fmt.Println(err, 2)
			err = nil
		}
		fmt.Println(i)
		time.Sleep(time.Minute)
	}
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan interface{})
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			fmt.Printf("\nReceived an interrupt, closing connection...\n")
			sc.Close()
			close(cleanupDone)
		}
	}()
	<-cleanupDone
}
