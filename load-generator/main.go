package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"
)

type OrderRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

func main() {
	orderServiceURL := os.Getenv("ORDER_SERVICE_URL")
	if orderServiceURL == "" {
		orderServiceURL = "http://localhost:8080"
	}

	products := []string{"prod-A", "prod-B", "prod-C", "prod-D"}

	log.Printf("Starting load generator -> %s/orders", orderServiceURL)

	ticker := time.NewTicker(500 * time.Millisecond)
	defer ticker.Stop()

	client := &http.Client{Timeout: 5 * time.Second}

	for range ticker.C {
		productID := products[rand.Intn(len(products))]
		quantity := int32(rand.Intn(10) + 1)

		req := OrderRequest{
			ProductID: productID,
			Quantity:  quantity,
		}

		body, err := json.Marshal(req)
		if err != nil {
			log.Printf("ERROR marshal: %v", err)
			continue
		}

		resp, err := client.Post(
			orderServiceURL+"/orders",
			"application/json",
			bytes.NewReader(body),
		)
		if err != nil {
			log.Printf("ERROR POST /orders: %v", err)
			continue
		}
		resp.Body.Close()

		fmt.Printf("order sent: product=%s qty=%d status=%d\n", productID, quantity, resp.StatusCode)
	}
}
