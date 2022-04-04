package libr

import (
	"errors"
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

type Str struct {
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

func NewStrGen() *Str {
	var i = rand.Int()
	var D = NewDeliveryGen()
	var P = NewPaymentGen()
	var I1, I2, I3 = NewItemGen(), NewItemGen(), NewItemGen()
	var I = []Item{*I1, *I2, *I3}
	return &Str{OrderUID: "orderUID" + strconv.Itoa(i), TrackNumber: "trackNumber" + strconv.Itoa(i), Entry: "entry" + strconv.Itoa(i), Deliveries: *D, Pays: *P, Items: I, Locale: "locale" + strconv.Itoa(i), InternalSignature: "internalSignature" + strconv.Itoa(i), CustomerID: "customerID" + strconv.Itoa(i), DeliveryService: "deliveryService" + strconv.Itoa(i), Shardkey: "shardkey" + strconv.Itoa(i), SmID: i, DateCreated: time.Now().Add(time.Duration(i) * time.Millisecond), OofShard: "oofShard" + strconv.Itoa(i)}
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

type Database struct {
	cache *Cache
}

// чисто красивая структура для подключения к бд через pgx
type Connector struct {
	Uname  string
	Pass   string
	Host   string
	Port   string
	Dbname string
}
