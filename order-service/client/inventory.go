package client

import (
	"context"

	inventory "github.com/demo/order-service/inventory"
	"google.golang.org/grpc"
)

type InventoryClient struct {
	client inventory.InventoryClient
}

func NewInventoryClient(conn *grpc.ClientConn) *InventoryClient {
	return &InventoryClient{client: inventory.NewInventoryClient(conn)}
}

func (c *InventoryClient) CheckStock(ctx context.Context, productID string, quantity int32) (*inventory.StockResponse, error) {
	return c.client.CheckStock(ctx, &inventory.StockRequest{
		ProductId: productID,
		Quantity:  quantity,
	})
}
