package main

import (
	"fmt"
	"time"

	"github.com/google/btree"
)

type Order struct {
	OrderID   string
	Price     float64
	Quantity  float64
	Timestamp time.Time
	Side      string // "buy" or "sell"
}

type PriceLevel struct {
	Price  float64
	Orders []*Order
	Side   string // "buy" or "sell"
}

func (p PriceLevel) Less(than btree.Item) bool {
	o := than.(PriceLevel)
	if p.Side == "buy" {
		// For buy orders, higher prices come first.
		return p.Price > o.Price
	} else {
		// For sell orders, lower prices come first.
		return p.Price < o.Price
	}
}

type OrderBook struct {
	BuyTree         *btree.BTree      // Stores PriceLevels with Side "buy"
	SellTree        *btree.BTree      // Stores PriceLevels with Side "sell"
	Orders          map[string]*Order // Map from OrderID to Order pointer
	LastTradedPrice float64           // Stores the last traded price
}

func NewOrderBook() *OrderBook {
	return &OrderBook{
		BuyTree:  btree.New(3),
		SellTree: btree.New(3),
		Orders:   make(map[string]*Order),
	}
}

func (ob *OrderBook) AddOrder(order *Order) {
	ob.Orders[order.OrderID] = order

	priceLevel := PriceLevel{
		Price: order.Price,
		Side:  order.Side,
	}

	var tree *btree.BTree
	if order.Side == "buy" {
		tree = ob.BuyTree
	} else {
		tree = ob.SellTree
	}

	item := tree.Get(priceLevel)
	if item != nil {
		existingLevel := item.(PriceLevel)
		existingLevel.Orders = append(existingLevel.Orders, order)
		tree.ReplaceOrInsert(existingLevel)
	} else {
		priceLevel.Orders = []*Order{order}
		tree.ReplaceOrInsert(priceLevel)
	}
}

func (ob *OrderBook) RemoveOrder(orderID string) error {
	order, exists := ob.Orders[orderID]
	if !exists {
		return fmt.Errorf("order not found")
	}

	ob.removeOrder(order)
	delete(ob.Orders, orderID)
	return nil
}

func (ob *OrderBook) removeOrder(order *Order) {
	priceLevel := PriceLevel{
		Price: order.Price,
		Side:  order.Side,
	}

	var tree *btree.BTree
	if order.Side == "buy" {
		tree = ob.BuyTree
	} else {
		tree = ob.SellTree
	}

	item := tree.Get(priceLevel)
	if item != nil {
		existingLevel := item.(PriceLevel)
		for idx, ord := range existingLevel.Orders {
			if ord.OrderID == order.OrderID {
				// Remove from list
				existingLevel.Orders = append(existingLevel.Orders[:idx], existingLevel.Orders[idx+1:]...)
				if len(existingLevel.Orders) == 0 {
					tree.Delete(existingLevel)
				} else {
					tree.ReplaceOrInsert(existingLevel)
				}
				break
			}
		}
	}
}

func (ob *OrderBook) ModifyOrderPrice(orderID string, newPrice float64) error {
	order, exists := ob.Orders[orderID]
	if !exists {
		return fmt.Errorf("order not found")
	}

	// Remove order from current price level
	ob.removeOrder(order)

	// Update order's price
	order.Price = newPrice

	// Re-insert the order into the appropriate tree
	ob.AddOrder(order)
	return nil
}

func (ob *OrderBook) ModifyOrderSide(orderID string, newSide string) error {
	if newSide != "buy" && newSide != "sell" {
		return fmt.Errorf("invalid side")
	}

	order, exists := ob.Orders[orderID]
	if !exists {
		return fmt.Errorf("order not found")
	}

	// Remove order from current side
	ob.removeOrder(order)

	// Update order's side
	order.Side = newSide

	// Re-insert the order into the appropriate tree
	ob.AddOrder(order)
	return nil
}

func (ob *OrderBook) ModifyOrderQuantity(orderID string, newQuantity float64) error {
	if newQuantity < 0 {
		return fmt.Errorf("invalid quantity")
	}

	order, exists := ob.Orders[orderID]
	if !exists {
		return fmt.Errorf("order not found")
	}

	// Remove order if quantity is zero
	if newQuantity == 0 {
		ob.removeOrder(order)
		delete(ob.Orders, orderID)
	} else {
		// Update quantity
		order.Quantity = newQuantity
		// No need to re-insert since the order's position doesn't change
	}
	return nil
}

func (ob *OrderBook) MatchOrders() {
	for {
		buyItem := ob.BuyTree.Min()
		sellItem := ob.SellTree.Min()
		if buyItem == nil || sellItem == nil {
			break
		}

		highestBuy := buyItem.(PriceLevel)
		lowestSell := sellItem.(PriceLevel)

		if highestBuy.Price >= lowestSell.Price {
			// Execute trade between highestBuy and lowestSell orders
			buyOrder := highestBuy.Orders[0]
			sellOrder := lowestSell.Orders[0]

			// Determine trade price (assuming trades occur at the sell order's price)
			tradePrice := sellOrder.Price

			// Update LastTradedPrice
			ob.LastTradedPrice = tradePrice

			// Determine trade quantity
			tradeQuantity := min(buyOrder.Quantity, sellOrder.Quantity)

			fmt.Printf("Trade executed: Buy Order %s and Sell Order %s at Price %.2f for Quantity %.2f\n",
				buyOrder.OrderID, sellOrder.OrderID, tradePrice, tradeQuantity)

			// Update orders
			buyOrder.Quantity -= tradeQuantity
			sellOrder.Quantity -= tradeQuantity

			// Remove orders if fully filled
			if buyOrder.Quantity == 0 {
				ob.removeOrder(buyOrder)
				delete(ob.Orders, buyOrder.OrderID)
			}

			if sellOrder.Quantity == 0 {
				ob.removeOrder(sellOrder)
				delete(ob.Orders, sellOrder.OrderID)
			}
		} else {
			break // No matching prices
		}
	}
}

func (ob *OrderBook) GetBestBid() (float64, bool) {
	if ob.BuyTree.Len() == 0 {
		return 0, false // No bids available
	}
	item := ob.BuyTree.Min()
	bestBidItem := item.(PriceLevel)
	return bestBidItem.Price, true
}

func (ob *OrderBook) GetBestAsk() (float64, bool) {
	if ob.SellTree.Len() == 0 {
		return 0, false // No asks available
	}
	item := ob.SellTree.Min()
	bestAskItem := item.(PriceLevel)
	return bestAskItem.Price, true
}

func (ob *OrderBook) GetMidPrice() (float64, bool) {
	bestBid, hasBid := ob.GetBestBid()
	bestAsk, hasAsk := ob.GetBestAsk()
	if hasBid && hasAsk {
		return (bestBid + bestAsk) / 2, true
	}
	return 0, false // Cannot calculate mid-price if either side is empty
}

func (ob *OrderBook) GetCurrentMarketPrice() (float64, bool) {
	if ob.LastTradedPrice != 0 {
		return ob.LastTradedPrice, true
	}
	midPrice, ok := ob.GetMidPrice()
	if ok {
		return midPrice, true
	}
	// If no trades have occurred and mid-price cannot be calculated
	return 0, false
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

func (ob *OrderBook) DisplayOrderBook() {
	fmt.Println("Order Book:")
	fmt.Println("Buy Orders:")
	ob.BuyTree.Ascend(func(i btree.Item) bool {
		item := i.(PriceLevel)
		for _, ord := range item.Orders {
			fmt.Printf("OrderID: %s, Price: %.2f, Quantity: %.2f\n", ord.OrderID, ord.Price, ord.Quantity)
		}
		return true
	})
	fmt.Println("Sell Orders:")
	ob.SellTree.Ascend(func(i btree.Item) bool {
		item := i.(PriceLevel)
		for _, ord := range item.Orders {
			fmt.Printf("OrderID: %s, Price: %.2f, Quantity: %.2f\n", ord.OrderID, ord.Price, ord.Quantity)
		}
		return true
	})
	fmt.Println("------------------------------")
}

func main() {
	// Initialize the order book
	orderBook := NewOrderBook()

	// Add some orders
	order1 := &Order{
		OrderID:   "B1",
		Price:     100.0,
		Quantity:  10,
		Timestamp: time.Now(),
		Side:      "buy",
	}
	orderBook.AddOrder(order1)

	order2 := &Order{
		OrderID:   "S1",
		Price:     105.0,
		Quantity:  5,
		Timestamp: time.Now(),
		Side:      "sell",
	}
	orderBook.AddOrder(order2)

	order3 := &Order{
		OrderID:   "B2",
		Price:     102.0,
		Quantity:  7,
		Timestamp: time.Now(),
		Side:      "buy",
	}
	orderBook.AddOrder(order3)

	order4 := &Order{
		OrderID:   "S2",
		Price:     99.0,
		Quantity:  8,
		Timestamp: time.Now(),
		Side:      "sell",
	}
	orderBook.AddOrder(order4)

	// Display the order book
	orderBook.DisplayOrderBook()

	// Match orders
	orderBook.MatchOrders()

	// Get current market price
	marketPrice, ok := orderBook.GetCurrentMarketPrice()
	if ok {
		fmt.Printf("Current Market Price: %.2f\n", marketPrice)
	} else {
		fmt.Println("Market price is not available.")
	}

	// Display the order book after matching
	orderBook.DisplayOrderBook()

	// Modify an order's price
	err := orderBook.ModifyOrderPrice("B2", 98.0)
	if err != nil {
		fmt.Println(err)
	}

	// Modify an order's side
	err = orderBook.ModifyOrderSide("S1", "buy")
	if err != nil {
		fmt.Println(err)
	}

	// Modify an order's quantity
	err = orderBook.ModifyOrderQuantity("B1", 0)
	if err != nil {
		fmt.Println(err)
	}

	// Display the order book after modifications
	orderBook.DisplayOrderBook()

	// Match orders again
	orderBook.MatchOrders()

	// Get current market price after matching
	marketPrice, ok = orderBook.GetCurrentMarketPrice()
	if ok {
		fmt.Printf("Current Market Price: %.2f\n", marketPrice)
	} else {
		fmt.Println("Market price is not available.")
	}

	// Final state of the order book
	orderBook.DisplayOrderBook()
}
