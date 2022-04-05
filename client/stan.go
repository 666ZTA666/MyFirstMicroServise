package main

import (
	"WB1/libr"
	"context"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	stan "github.com/nats-io/stan.go"
	"net/http"
	"os"
	"os/signal"
	"time"
)

func main() {
	fmt.Println(time.Now(), "Work is begining.")
	var err error
	var ServStruck = libr.NewSkz(libr.Connector{Uname: "postgres", Pass: "1488", Host: "localhost", Port: "5432", Dbname: "wbbase"}, 15*time.Minute, 3*time.Minute)
	//todo запись в кэш из БД
	//err = ServStruck.InitSomeCache()
	if err != nil {
		fmt.Println(time.Now(), "caching data going wrong:", err)
	}
	// собираем из структуры строку для подключения к бд
	StringOfConnectionToDataBase := ServStruck.Con.GetPGSQL()
	// Подключаемся к серверу сообщений
	ServStruck.StreamConn, err = stan.Connect("test-cluster", "client-123", stan.NatsURL("0.0.0.0:4222"))
	if err != nil {
		fmt.Println("Can't connect to cluster", err)
		err = nil
	}
	fmt.Println(time.Now(), "Connected to cluster. Success")
	//Подключаемся к БД
	ServStruck.Pool, err = pgxpool.Connect(context.TODO(), StringOfConnectionToDataBase)
	if err != nil {
		fmt.Println("Unable to connect to database:", err)
		err = nil
	}
	fmt.Println(time.Now(), "Connected to Database. Success")

	// Подписка на канал в который передано дефолтное название и метод для обработки сообщений.
	ServStruck.StreamSubscribe, err = ServStruck.StreamConn.Subscribe("foo", ServStruck.MesageHandler)
	if err != nil {
		fmt.Println("Can't subscribe to chanel:", err)
		err = nil
	}
	fmt.Println(time.Now(), "Subscribe is done. Succsess")
	//начинаем писать веб часть ниже

	fmt.Println(time.Now(), "Listening on port: 3000")
	http.HandleFunc("/", ServStruck.OrderHandler)
	err = http.ListenAndServe(":3000", nil)
	if err != nil {
		fmt.Println(time.Now(), "\"http.ListenAndServe\" have some err to you", err)
	}

	// вот эту красивую закрывашку Я взял из примеров stan, общий механизм в том, чтобы чтение продолжалось пока Ctrl+С не закроет программу.
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan interface{})
	signal.Notify(signalChan, os.Interrupt)
	go func() { // в отдельной горутине работает отлов сигнала об остановке работы программы
		// в случае если поймает, то отписывается, закрывает подключение к серверу и заканчивает работу
		for range signalChan {
			fmt.Println(time.Now(), "Received an interrupt, unsubscribing and closing connection...")
			err := ServStruck.StreamSubscribe.Unsubscribe()
			if err != nil {
				fmt.Println(time.Now(), "trouble in unsubscribing:", err)
			}
			err = ServStruck.StreamConn.Close()
			if err != nil {
				fmt.Println(time.Now(), "Closing connection with stream server going wrong", err)
			}
			ServStruck.Pool.Close()
			close(cleanupDone)
		}
	}()
	<-cleanupDone
	fmt.Println(time.Now(), "Exiting, glhf")
}
