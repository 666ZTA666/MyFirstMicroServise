package main

import (
	"WB1/libr"
	"encoding/json"
	"fmt"
	stan "github.com/nats-io/stan.go"
	"os"
	"os/signal"
)

func main() {
	sc, err := stan.Connect("test-cluster", "client-123", stan.NatsURL("0.0.0.0:4222"))
	if err != nil {
		fmt.Println(err, 1)
		err = nil
	}
	var sub stan.Subscription
	//var chStr = make(chan Str, 1)
	sub, err = sc.Subscribe("foo", func(m *stan.Msg) {
		var t libr.Str
		err := json.Unmarshal(m.Data, &t)
		if err != nil {
			fmt.Println(err, "Json")
		}
		fmt.Println(t.OrderUID)
	})
	if err != nil {
		fmt.Println(err, 2)
		err = nil
	}

	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan interface{})
	signal.Notify(signalChan, os.Interrupt)
	go func() {
		for range signalChan {
			fmt.Printf("\nReceived an interrupt, unsubscribing and closing connection...\n\n")
			sub.Unsubscribe()
			sc.Close()
			close(cleanupDone)
		}
	}()
	<-cleanupDone
}
