package libr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	"github.com/nats-io/stan.go"
	"math/rand"
	"strconv"
	"sync"
	"time"
)

// структуры и методы для работы со входящим json
type Delivery struct {
	Name    string `json:"name"`
	Phone   string `json:"phone"`
	Zip     string `json:"zip"`
	City    string `json:"city"`
	Address string `json:"address"`
	Region  string `json:"region"`
	Email   string `json:"email"`
}

func NewDeliveryGen() *Delivery {
	var i = rand.Int()
	return &Delivery{Name: "name" + strconv.Itoa(i), Phone: "phone" + strconv.Itoa(i), Zip: "zip" + strconv.Itoa(i), City: "city" + strconv.Itoa(i), Address: "address" + strconv.Itoa(i), Region: "region" + strconv.Itoa(i), Email: "email" + strconv.Itoa(i)}
}

type Payment struct {
	Transaction  string `json:"transaction"`
	RequestID    string `json:"request_id"`
	Currency     string `json:"currency"`
	Provider     string `json:"provider"`
	Amount       int    `json:"amount"`
	PaymentDt    int    `json:"payment_dt"`
	Bank         string `json:"bank"`
	DeliveryCost int    `json:"delivery_cost"`
	GoodsTotal   int    `json:"goods_total"`
	CustomFee    int    `json:"custom_fee"`
}

func NewPaymentGen() *Payment {
	var i = rand.Int()
	return &Payment{Transaction: "transaction" + strconv.Itoa(i), RequestID: "requestID" + strconv.Itoa(i), Currency: "currency" + strconv.Itoa(i), Provider: "provider" + strconv.Itoa(i), Amount: i, PaymentDt: i, Bank: "bank" + strconv.Itoa(i), DeliveryCost: i, GoodsTotal: i, CustomFee: i}
}

type Item struct {
	ChrtID      int    `json:"chrt_id"`
	TrackNumber string `json:"track_number"`
	Price       int    `json:"price"`
	Rid         string `json:"rid"`
	Name        string `json:"name"`
	Sale        int    `json:"sale"`
	Size        string `json:"size"`
	TotalPrice  int    `json:"total_price"`
	NmID        int    `json:"nm_id"`
	Brand       string `json:"brand"`
	Status      int    `json:"status"`
}

func NewItemGen() *Item {
	var i = rand.Int()
	return &Item{ChrtID: i, TrackNumber: "trackNumber" + strconv.Itoa(i), Price: i, Rid: "rid" + strconv.Itoa(i), Name: "name" + strconv.Itoa(i), Sale: i, Size: "size" + strconv.Itoa(i), TotalPrice: i, NmID: i, Brand: "brand" + strconv.Itoa(i), Status: i}
}

type Order struct {
	OrderUID          string    `json:"order_uid"`
	TrackNumber       string    `json:"track_number"`
	Entry             string    `json:"entry"`
	Deliveries        Delivery  `json:"delivery"`
	Pays              Payment   `json:"payment"`
	Items             []Item    `json:"items"`
	Locale            string    `json:"locale"`
	InternalSignature string    `json:"internal_signature"`
	CustomerID        string    `json:"customer_id"`
	DeliveryService   string    `json:"delivery_service"`
	Shardkey          string    `json:"shardkey"`
	SmID              int       `json:"sm_id"`
	DateCreated       time.Time `json:"date_created"`
	OofShard          string    `json:"oof_shard"`
}

func NewStrGen() *Order {
	var i = rand.Int()
	var D = NewDeliveryGen()
	var P = NewPaymentGen()
	var I1, I2, I3 = NewItemGen(), NewItemGen(), NewItemGen()
	var I = []Item{*I1, *I2, *I3}
	return &Order{OrderUID: "orderUID" + strconv.Itoa(i), TrackNumber: "trackNumber" + strconv.Itoa(i), Entry: "entry" + strconv.Itoa(i), Deliveries: *D, Pays: *P, Items: I, Locale: "locale" + strconv.Itoa(i), InternalSignature: "internalSignature" + strconv.Itoa(i), CustomerID: "customerID" + strconv.Itoa(i), DeliveryService: "deliveryService" + strconv.Itoa(i), Shardkey: "shardkey" + strconv.Itoa(i), SmID: i, DateCreated: time.Now().Add(time.Duration(i) * time.Millisecond), OofShard: "oofShard" + strconv.Itoa(i)}
}

//кэширование честно сжиженое с хабра и переписанное со времени на количество(?)
type ItemForCache struct {
	Value      interface{}
	Created    time.Time
	Expiration int64
}
type Cache struct {
	sync.RWMutex
	defaultExpiration time.Duration
	cleanupInterval   time.Duration
	items             map[string]ItemForCache
}

func NewCatch(defaultExpiration, cleanupInterval time.Duration) *Cache {
	items := make(map[string]ItemForCache)
	cache := Cache{
		items:             items,
		defaultExpiration: defaultExpiration,
		cleanupInterval:   cleanupInterval,
	}
	// Если интервал очистки больше 0, запускаем GC (удаление устаревших элементов)
	if cleanupInterval > 0 {
		cache.StartGC() // данный метод рассматривается ниже
	}
	return &cache
}
func (c *Cache) Set(key string, value interface{}, duration time.Duration) {
	var expiration int64
	if duration == 0 {
		duration = c.defaultExpiration
	}

	if duration > 0 {
		expiration = time.Now().Add(duration).UnixNano()
	}
	c.Lock()
	defer c.Unlock()
	if _, ok := c.items[key]; ok == true {
		fmt.Println("Key is not unique. Thai is already data for this key. Overwriting is not allowed")
		return
	}
	c.items[key] = ItemForCache{
		Value:      value,
		Expiration: expiration,
		Created:    time.Now(),
	}
}
func (c *Cache) Get(key string) (interface{}, bool) {
	c.RLock()
	defer c.RUnlock()
	item, found := c.items[key]
	// ключ не найден
	if !found {
		return nil, false
	}
	// Проверка на установку времени истечения, в противном случае он бессрочный
	if item.Expiration > 0 {
		// Если в момент запроса кеш устарел возвращаем nil
		if time.Now().UnixNano() > item.Expiration {
			return nil, false
		}
	}
	return item.Value, true
}
func (c *Cache) Delete(key string) error {
	c.Lock()
	defer c.Unlock()
	if _, found := c.items[key]; !found {
		return errors.New("Key not found")
	}
	delete(c.items, key)
	return nil
}
func (c *Cache) StartGC() {
	go c.GC()
}
func (c *Cache) GC() {
	for {
		// ожидаем время установленное в cleanupInterval
		<-time.After(c.cleanupInterval)
		if c.items == nil {
			return
		}
		// Ищем элементы с истекшим временем жизни и удаляем из хранилища
		if keys := c.expiredKeys(); len(keys) != 0 {
			c.clearItems(keys)

		}
	}
}
func (c *Cache) expiredKeys() (keys []string) {
	c.RLock()
	defer c.RUnlock()
	for k, i := range c.items {
		if time.Now().UnixNano() > i.Expiration && i.Expiration > 0 {
			keys = append(keys, k)
		}
	}
	return
}
func (c *Cache) clearItems(keys []string) {
	c.Lock()
	defer c.Unlock()
	for _, k := range keys {
		delete(c.items, k)
	}
}

// чисто красивая структура для подключения к бд через pgx
type Connector struct {
	Uname  string
	Pass   string
	Host   string
	Port   string
	Dbname string
}

// метод для получения строки подключения к pgsql
func (con Connector) GetPGSQL() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", con.Uname, con.Pass, con.Host, con.Port, con.Dbname)
}

// структура типа servise которая содержит в себе данные о бд и кэше минимум
// метод func(m *stan.Msg) который имеет доступ к БД и кэшу
type Skz struct {
	Con             Connector
	Zakaz           Order
	Pool            *pgxpool.Pool
	Cash            *Cache
	StreamConn      stan.Conn
	StreamSubscribe stan.Subscription
}

func NewSkz(con Connector, defaultExpiration, cleanupInterval time.Duration) *Skz {
	return &Skz{Con: con, Cash: NewCatch(defaultExpiration, cleanupInterval)}
}

func (o *Skz) MesageHandler(m *stan.Msg) {
	err := json.Unmarshal(m.Data, &o.Zakaz)
	var ResultDelivery, ResultPayment, ResultOrder, ResultItems string
	if err != nil {
		fmt.Println(err, "Json") //вообще непонятно на что Я полагаюсь выводя эту ошибку, TODO валидатор в телеге
	}
	o.Cash.Set(o.Zakaz.OrderUID, o.Zakaz, 5*time.Minute)
	fmt.Println(o.Zakaz.OrderUID, "putted in cache")

	query := "INSERT INTO delivery (del_name, Phone, Zip, City, Address, Region, Email)	Values ($1, $2, $3, $4, $5, $6, $7) returning del_id"
	err = o.Pool.QueryRow(context.TODO(), query, o.Zakaz.Deliveries.Name, o.Zakaz.Deliveries.Phone, o.Zakaz.Deliveries.Zip, o.Zakaz.Deliveries.City, o.Zakaz.Deliveries.Address, o.Zakaz.Deliveries.Region, o.Zakaz.Deliveries.Email).Scan(&ResultDelivery)
	if err != nil {
		fmt.Println("Insert to Delivery failed:", err)

	}
	fmt.Println("delivery =", ResultDelivery)

	query = "INSERT INTO payment (Transaction, RequestID, Currency, Provider, Amount, PaymentDt, Bank, DeliveryCost, GoodsTotal, CustomFee)	Values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) returning pay_id"
	err = o.Pool.QueryRow(context.TODO(), query, o.Zakaz.Pays.Transaction, o.Zakaz.Pays.RequestID, o.Zakaz.Pays.Currency, o.Zakaz.Pays.Provider, o.Zakaz.Pays.Amount, o.Zakaz.Pays.PaymentDt, o.Zakaz.Pays.Bank, o.Zakaz.Pays.DeliveryCost, o.Zakaz.Pays.GoodsTotal, o.Zakaz.Pays.CustomFee).Scan(&ResultPayment)
	if err != nil {
		fmt.Println("Insert to Payment failed:", err)
	}
	fmt.Println("payment =", ResultPayment)

	it := make([]int, len(o.Zakaz.Items))

	for i := 0; i < len(o.Zakaz.Items); i++ {
		it[i] = o.Zakaz.Items[i].ChrtID
	}

	query = "INSERT INTO order (OrderUID, TrackNumber, Entry, Deliveries, Pays, Items, Locale, InternalSignature, CustomerID, DeliveryService, Shardkey, SmID, DateCreated, OofShard)	Values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) returning OrderUID"
	err = o.Pool.QueryRow(context.TODO(), query, o.Zakaz.OrderUID, o.Zakaz.TrackNumber, o.Zakaz.Entry, ResultDelivery, ResultPayment, it, o.Zakaz.Locale, o.Zakaz.InternalSignature, o.Zakaz.CustomerID, o.Zakaz.DeliveryService, o.Zakaz.Shardkey, o.Zakaz.SmID, o.Zakaz.DateCreated, o.Zakaz.OofShard).Scan(&ResultOrder)
	if err != nil {
		fmt.Println("Insert to Order failed:", err)
	}
	fmt.Println("Order =", ResultOrder)

	for j := 0; j < len(o.Zakaz.Items); j++ {
		query = "INSERT INTO items (ChrtID, TrackNumber, Price, Rid, Item_name, Sale, Size, TotalPrice, NmID, Brand, Status, Orderid)	Values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) returning ChrtID"
		err = o.Pool.QueryRow(context.TODO(), query, o.Zakaz.Items[j].ChrtID, o.Zakaz.Items[j].TrackNumber, o.Zakaz.Items[j].Price, o.Zakaz.Items[j].Rid, o.Zakaz.Items[j].Name, o.Zakaz.Items[j].Sale, o.Zakaz.Items[j].Size, o.Zakaz.Items[j].TotalPrice, o.Zakaz.Items[j].NmID, o.Zakaz.Items[j].Brand, o.Zakaz.Items[j].Status, o.Zakaz.OrderUID).Scan(&ResultItems)
		if err != nil {
			fmt.Println("Insert to Payment failed:", err)
		}
		fmt.Println("item =", ResultItems)
	}
}
