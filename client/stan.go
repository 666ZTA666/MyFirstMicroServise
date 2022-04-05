package main

import (
	"WB1/libr"
	"context"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	stan "github.com/nats-io/stan.go"
	"os"
	"os/signal"
	"time"
)

// Объявляем глобальную структуру skz
var ServStruck = libr.NewSkz(libr.Connector{Uname: "postgres", Pass: "1488", Host: "localhost", Port: "5432", Dbname: "wbbase"}, 15*time.Minute, 3*time.Minute)

func main() {
	// собираем из структуры строку для подключения к бд
	StringOfConnectionToDataBase := ServStruck.Con.GetPGSQL()
	// Подключаемся к серверу сообщений
	var err error
	ServStruck.StreamConn, err = stan.Connect("test-cluster", "client-123", stan.NatsURL("0.0.0.0:4222"))
	if err != nil {
		fmt.Println("Can't connect to cluster", err)
		err = nil
	}
	//Подключаемся к БД
	ServStruck.Pool, err = pgxpool.Connect(context.TODO(), StringOfConnectionToDataBase)
	if err != nil {
		fmt.Println("Unable to connect to database:", err)
		err = nil
	}

	// Подписка на канал в который передано дефолтное название и метод для обработки сообщений.
	ServStruck.StreamSubscribe, err = ServStruck.StreamConn.Subscribe("foo", ServStruck.MesageHandler)
	if err != nil {
		fmt.Println("Can't subscribe to chanel:", err)
		err = nil
	}

	//todo вот сюда мы заебошим чтение из локалхоста и вывод данных из мапы

	// вот эту красивую закрывашку Я взял из примеров stan, общий механизм в том, чтобы чтение продолжалось пока Ctrl+С не закроет программу.
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan interface{})
	signal.Notify(signalChan, os.Interrupt)
	go func() { // в отдельной горутине работает отлов сигнала об остановке работы программы
		// в случае если поймает, то отписывается, закрывает подключение к серверу и заканчивает работу
		for range signalChan {
			fmt.Printf("Received an interrupt, unsubscribing and closing connection...\n")
			ServStruck.StreamSubscribe.Unsubscribe() // не отловленные ошибки №1
			ServStruck.StreamConn.Close()            // не отловленные ошибки №2
			ServStruck.Pool.Close()
			close(cleanupDone)
		}
	}()
	<-cleanupDone
}
