package libr

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/jackc/pgx/v4/pgxpool"
	stan "github.com/nats-io/stan.go"
	"html/template"
	"math/rand"
	"net/http"
	"strconv"
	"sync"
	"time"
)

// Структуры и методы для работы со входящим json
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

func NewItemsGen(number int) []Item {
	It := make([]Item, number)
	for number > 0 {
		var i = rand.Int()
		It[number] = Item{ChrtID: i, TrackNumber: "trackNumber" + strconv.Itoa(i), Price: i, Rid: "rid" + strconv.Itoa(i), Name: "name" + strconv.Itoa(i), Sale: i, Size: "size" + strconv.Itoa(i), TotalPrice: i, NmID: i, Brand: "brand" + strconv.Itoa(i), Status: i}
		number--
	}
	return It
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
	var j = rand.Intn(10)
	var D = NewDeliveryGen()
	var P = NewPaymentGen()
	var I = NewItemsGen(j)
	return &Order{OrderUID: "orderUID" + strconv.Itoa(i), TrackNumber: "trackNumber" + strconv.Itoa(i), Entry: "entry" + strconv.Itoa(i), Deliveries: *D, Pays: *P, Items: I, Locale: "locale" + strconv.Itoa(i), InternalSignature: "internalSignature" + strconv.Itoa(i), CustomerID: "customerID" + strconv.Itoa(i), DeliveryService: "deliveryService" + strconv.Itoa(i), Shardkey: "shardkey" + strconv.Itoa(i), SmID: i, DateCreated: time.Now().Add(time.Duration(i) * time.Millisecond), OofShard: "oofShard" + strconv.Itoa(i)}
}

//Кэширование честно сжиженое с хабра
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
		// Ищем элементы с истёкшим временем жизни и удаляем из хранилища
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

type Connector struct {
	Uname  string
	Pass   string
	Host   string
	Port   string
	Dbname string
}

func (con Connector) GetPGSQL() string {
	return fmt.Sprintf("postgresql://%s:%s@%s:%s/%s", con.Uname, con.Pass, con.Host, con.Port, con.Dbname)
}

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

func (o *Skz) InitSomeCache() error {

	return nil
}

func (o *Skz) FromDbToCacheByKey() error {
	if o.Zakaz.OrderUID == "" {
		return fmt.Errorf("key is empty")
	}
	var DelId, PayId string
	query := "Select (TrackNumber, Entry, Deliveries, Pays, Items, Locale, InternalSignature, CustomerID, DeliveryService, Shardkey, SmID, DateCreated, OofShard) from orders where orderUID = $1"
	rows, err := o.Pool.Query(context.TODO(), query, o.Zakaz.OrderUID)
	if err != nil {
		fmt.Println(time.Now(), "Select from Order failed:", err)
		return err
	}
	it := make([]int, 100)
	for rows.Next() {
		err = rows.Scan(&o.Zakaz.TrackNumber, &o.Zakaz.Entry, &DelId, &PayId, &it, &o.Zakaz.Locale, &o.Zakaz.InternalSignature, &o.Zakaz.CustomerID, &o.Zakaz.DeliveryService, &o.Zakaz.Shardkey, &o.Zakaz.SmID, &o.Zakaz.DateCreated, &o.Zakaz.OofShard)
		if err != nil {
			fmt.Println(time.Now(), "Scanning rows from selected order failed:", err)
			return err
		}
	}
	fmt.Println(time.Now(), "Order =", o.Zakaz.OrderUID)

	query = "Select (ChrtID, TrackNumber, Price, Rid, Item_name, Sale, Size, TotalPrice, NmID, Brand, Status) from items where orderid = $1"
	rows, err = o.Pool.Query(context.TODO(), query, o.Zakaz.OrderUID)
	if err != nil {
		fmt.Println(time.Now(), "Select from Items failed:", err)
		return err
	}
	for rows.Next() {
		var j int
		var i Item
		err = rows.Scan(&i.ChrtID, &i.TrackNumber, &i.Price, &i.Rid, &i.Name, &i.Sale, &i.Size, &i.TotalPrice, &i.NmID, &i.Brand, &i.Status)
		if err != nil {
			fmt.Println(time.Now(), "Scanning rows from selected items failed:", err)
			return err
		}
		o.Zakaz.Items = append(o.Zakaz.Items, i)
		fmt.Println(time.Now(), "item =", o.Zakaz.Items[j].ChrtID)
		j++
	}
	query = "Select (del_name, Phone, Zip, City, Address, Region, Email) from delivery where del_id = $1"
	rows, err = o.Pool.Query(context.TODO(), query, DelId)
	if err != nil {
		fmt.Println(time.Now(), "Select from Delivery failed:", err)
		return err
	}
	for rows.Next() {
		err = rows.Scan(&o.Zakaz.Deliveries.Name, &o.Zakaz.Deliveries.Phone, &o.Zakaz.Deliveries.Zip, &o.Zakaz.Deliveries.City, &o.Zakaz.Deliveries.Address, &o.Zakaz.Deliveries.Region, &o.Zakaz.Deliveries.Email)
		if err != nil {
			fmt.Println(time.Now(), "Scanning rows from selected delivery failed:", err)
			return err
		}
	}
	fmt.Println(time.Now(), "delivery =", DelId)

	query = "select (Transaction, RequestID, Currency, Provider, Amount, PaymentDt, Bank, DeliveryCost, GoodsTotal, CustomFee) From payment where pay_id = $1"
	rows, err = o.Pool.Query(context.TODO(), query, PayId)
	if err != nil {
		fmt.Println(time.Now(), "Select from Payment failed:", err)
		return err
	}
	for rows.Next() {
		err = rows.Scan(&o.Zakaz.Pays.Transaction, &o.Zakaz.Pays.RequestID, &o.Zakaz.Pays.Currency, &o.Zakaz.Pays.Provider, &o.Zakaz.Pays.Amount, &o.Zakaz.Pays.PaymentDt, &o.Zakaz.Pays.Bank, &o.Zakaz.Pays.DeliveryCost, &o.Zakaz.Pays.GoodsTotal, &o.Zakaz.Pays.CustomFee)
		if err != nil {
			fmt.Println(time.Now(), "Scanning rows from selected payments failed:", err)
			return err
		}
	}
	fmt.Println(time.Now(), "payment =", PayId)
	o.Cash.Set(o.Zakaz.OrderUID, o.Zakaz, 5*time.Minute)
	return nil
}

func (o *Skz) MesageHandler(m *stan.Msg) {
	err := json.Unmarshal(m.Data, &o.Zakaz)
	var ResultDelivery, ResultPayment, ResultOrder string
	var ResultItems int
	if err != nil {
		fmt.Println(err, "Json") //вообще непонятно на что Я полагаюсь выводя эту ошибку, TODO валидатор в телеге
	}
	o.Cash.Set(o.Zakaz.OrderUID, o.Zakaz, 5*time.Minute)
	fmt.Println(time.Now(), o.Zakaz.OrderUID, "putted in cache")

	query := "INSERT INTO delivery (del_name, Phone, Zip, City, Address, Region, Email)	Values ($1, $2, $3, $4, $5, $6, $7) returning del_id"
	err = o.Pool.QueryRow(context.TODO(), query, o.Zakaz.Deliveries.Name, o.Zakaz.Deliveries.Phone, o.Zakaz.Deliveries.Zip, o.Zakaz.Deliveries.City, o.Zakaz.Deliveries.Address, o.Zakaz.Deliveries.Region, o.Zakaz.Deliveries.Email).Scan(&ResultDelivery)
	if err != nil {
		fmt.Println(time.Now(), "Insert to Delivery failed:", err)

	}
	fmt.Println(time.Now(), "delivery =", ResultDelivery)

	query = "INSERT INTO payment (Transaction, RequestID, Currency, Provider, Amount, PaymentDt, Bank, DeliveryCost, GoodsTotal, CustomFee)	Values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10) returning pay_id"
	err = o.Pool.QueryRow(context.TODO(), query, o.Zakaz.Pays.Transaction, o.Zakaz.Pays.RequestID, o.Zakaz.Pays.Currency, o.Zakaz.Pays.Provider, o.Zakaz.Pays.Amount, o.Zakaz.Pays.PaymentDt, o.Zakaz.Pays.Bank, o.Zakaz.Pays.DeliveryCost, o.Zakaz.Pays.GoodsTotal, o.Zakaz.Pays.CustomFee).Scan(&ResultPayment)
	if err != nil {
		fmt.Println(time.Now(), "Insert to Payment failed:", err)
	}
	fmt.Println(time.Now(), "payment =", ResultPayment)

	it := make([]int, len(o.Zakaz.Items))

	for i := 0; i < len(o.Zakaz.Items); i++ {
		it[i] = o.Zakaz.Items[i].ChrtID
	}

	query = "INSERT INTO orders (OrderUID, TrackNumber, Entry, Deliveries, Pays, Items, Locale, InternalSignature, CustomerID, DeliveryService, Shardkey, SmID, DateCreated, OofShard)	Values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14) returning OrderUID"
	err = o.Pool.QueryRow(context.TODO(), query, o.Zakaz.OrderUID, o.Zakaz.TrackNumber, o.Zakaz.Entry, ResultDelivery, ResultPayment, it, o.Zakaz.Locale, o.Zakaz.InternalSignature, o.Zakaz.CustomerID, o.Zakaz.DeliveryService, o.Zakaz.Shardkey, o.Zakaz.SmID, o.Zakaz.DateCreated, o.Zakaz.OofShard).Scan(&ResultOrder)
	if err != nil {
		fmt.Println(time.Now(), "Insert to Order failed:", err)
	}
	fmt.Println(time.Now(), "Order =", ResultOrder)

	for j := 0; j < len(o.Zakaz.Items); j++ {
		query = "INSERT INTO items (ChrtID, TrackNumber, Price, Rid, Item_name, Sale, Size, TotalPrice, NmID, Brand, Status, Orderid)	Values ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12) returning ChrtID"
		err = o.Pool.QueryRow(context.TODO(), query, o.Zakaz.Items[j].ChrtID, o.Zakaz.Items[j].TrackNumber, o.Zakaz.Items[j].Price, o.Zakaz.Items[j].Rid, o.Zakaz.Items[j].Name, o.Zakaz.Items[j].Sale, o.Zakaz.Items[j].Size, o.Zakaz.Items[j].TotalPrice, o.Zakaz.Items[j].NmID, o.Zakaz.Items[j].Brand, o.Zakaz.Items[j].Status, o.Zakaz.OrderUID).Scan(&ResultItems)
		if err != nil {
			fmt.Println(time.Now(), "Insert to Items failed:", err)
		}
		fmt.Println(time.Now(), "item =", ResultItems)
	}
}

func (o *Skz) OrderHandler(Writer http.ResponseWriter, Request *http.Request) {
	var err error
	switch Request.Method {
	case "GET":
		tmpl, err := template.ParseFiles("interface.html")
		if err != nil {
			http.Error(Writer, err.Error(), 400)
			return
		}
		err = tmpl.Execute(Writer, nil)
		if err != nil {
			http.Error(Writer, err.Error(), 400)
			return
		}
	case "POST":
		Ouid := Request.PostFormValue("order_uid")
		Value, found := o.Cash.Get(Ouid)
		if !found {
			_, err = fmt.Fprintf(Writer, "Reading from DB:\n")
			fmt.Println(time.Now(), "Reading from DB by request")
			if err != nil {
				fmt.Println(time.Now(), "Something wrong with \"fmt.Fprintf\"", err)
				return
			}
			o.Zakaz.OrderUID = Ouid
			err = o.FromDbToCacheByKey()
			if err != nil {
				fmt.Println(time.Now(), "Something Wrong with reading from DB", err)
				return
			}
			Value, _ = o.Cash.Get(o.Zakaz.OrderUID)
		} else {
			_, err = fmt.Fprintf(Writer, "Reading from Cache:\n")
			fmt.Println(time.Now(), "Reading from Cache")
			if err != nil {
				fmt.Println(time.Now(), "Something wrong with \"fmt.Fprintf\"", err)
				return
			}
		}
		JsonValue, err := json.MarshalIndent(Value, "", "\t")
		if err != nil {
			fmt.Println(time.Now(), "Marshaling JSON going wrong", err)
			return
		}
		_, err = fmt.Fprintf(Writer, string(JsonValue))
		fmt.Println(string(JsonValue))
		if err != nil {
			fmt.Println(time.Now(), "Something wrong with \"fmt.Fprintf\"", err)
			return
		}
		return
	}
}
