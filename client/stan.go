package main

import (
	"WB1/libr"
	"context"
	"encoding/json"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	stan "github.com/nats-io/stan.go"
	"os"
	"os/signal"
)

// объявляем глобальные переменные
var StreamSubscribe stan.Subscription                                                                        // подписка
var t libr.Str                                                                                               // заказ
var con = libr.Connector{Uname: "postgres", Pass: "1488", Host: "localhost", Port: "5432", Dbname: "wbbase"} // структура подключения к БД
var ResultFromDataBase string                                                                                // результат работы с БД

func main() {
	// собираем из структуры строку для подключения к бд
	StringOfConnectionToDataBase := fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", con.Uname, con.Pass, con.Host, con.Port, con.Dbname)
	// Подключаемся к серверу сообщений
	StreamConnect, err := stan.Connect("test-cluster", "client-123", stan.NatsURL("0.0.0.0:4222"))
	if err != nil {
		fmt.Println("Can't connect to cluster", err)
		err = nil
	}
	//Подключаемся к БД
	DatabasePoolOfConnections, err := pgxpool.Connect(context.TODO(), StringOfConnectionToDataBase)
	if err != nil {
		fmt.Println("Unable to connect to database:", err)
		err = nil
	}

	// Подписка на канал, ставим лайк, жмем на колокольчик и начинаем получать уведомления о новых сообщениях
	StreamSubscribe, err = StreamConnect.Subscribe("foo", func(m *stan.Msg) {
		// не знаю насколько законно оставлять всё в таком виде, возможно следующий комит уберет это гавно в отдельную функцию
		err := json.Unmarshal(m.Data, &t) // ну тут уже первые косяки, потому что пока что проверки никакой нет, и если в канал зашлют какашку то тут уже всё посыпется
		if err != nil {
			fmt.Println(err, "Json") //вообще непонятно на что Я полагаюсь выводя эту ошибку, TODO
		}
		// формируем запрос, вообще эту штуку Я скорее всего перепишу под интерфейс из pgx и превращу в метод Create todo
		// вообще надо бы по полной передавать все данные во все таблицы, а это около 4х запросов с insert во все базы
		query := "INSERT INTO delivery (del_name, Phone, Zip, City, Address, Region, Email)	Values ($1, $2, $3, $4, $5, $6, $7) returning del_id" //
		// тут мы в БД передаем запрос и выходную строку сканим в переменную результата работы БД
		err = DatabasePoolOfConnections.QueryRow(context.Background(), query, t.Deliveries.Name, t.Deliveries.Phone, t.Deliveries.Zip, t.Deliveries.City, t.Deliveries.Address, t.Deliveries.Region, t.Deliveries.Email).Scan(&ResultFromDataBase)
		if err != nil {
			fmt.Println("Insert to DataBase failed:", err)
		}
		fmt.Println(ResultFromDataBase) //консоль не лопнет, а мне будет понятно, как отработал Insert
	})
	if err != nil {
		fmt.Println("Can't subscribe to chanel:", err)
		err = nil
	}
	// на случай если в переменную что-то упало по ходу, опустошаем строку
	ResultFromDataBase = ""
	// вообще перед этим делом наверное лучше отписаться от канала, потому что получается конкурентное чтение и запись в/из БД
	query := "select (del_name, Phone, Zip, City, Address, Region, Email) from delivery " // вывести все доставки, переделаю в вывести все заказы
	err = DatabasePoolOfConnections.QueryRow(context.Background(), query).Scan(&ResultFromDataBase)
	if err != nil {
		fmt.Println("Select request failed:", err)
	}
	fmt.Println(ResultFromDataBase) //результат select'а Я хз что там в одну строку влезет, но посмотрим хотя бы сраотало или нет, если что есть query без row

	// вот эту красивую крокозябру Я взял из примеров stan, общий механизм в том, чтобы чтение продолжалось пока Ctrl+С не закроет программу.
	signalChan := make(chan os.Signal, 1)
	cleanupDone := make(chan interface{})
	signal.Notify(signalChan, os.Interrupt)
	go func() { // в отдельной горутине работает отлов сигнала о остановку работы программы
		// в случае если поймает, то отписывается, закрывает подключение к серверу и заканчивает работу
		for range signalChan {
			fmt.Printf("Received an interrupt, unsubscribing and closing connection...\n")
			StreamSubscribe.Unsubscribe() // неотловленные ошибки №1
			StreamConnect.Close()         // неотловленные ошибки №2
			DatabasePoolOfConnections.Close()
			close(cleanupDone)
		}
	}()
	<-cleanupDone
}
