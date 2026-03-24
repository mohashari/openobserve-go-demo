package server

import (
	"context"
	"log/slog"
	"time"

	"go.opentelemetry.io/contrib/bridges/otelslog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	inventory "github.com/demo/inventory-service/inventory"
)

// InventoryServer implements the gRPC InventoryServer interface.
type InventoryServer struct {
	inventory.UnimplementedInventoryServer
	stock              map[string]int32
	tracer             trace.Tracer
	meter              metric.Meter
	logger             *slog.Logger
	stockChecksTotal   metric.Int64Counter
	stockCheckDuration metric.Float64Histogram
}

// NewInventoryServer creates and initialises an InventoryServer.
func NewInventoryServer() (*InventoryServer, error) {
	tracer := otel.Tracer("inventory-service")
	meter := otel.Meter("inventory-service")

	stockChecksTotal, err := meter.Int64Counter(
		"inventory.stock_checks_total",
		metric.WithDescription("Total number of stock checks"),
	)
	if err != nil {
		return nil, err
	}

	stockCheckDuration, err := meter.Float64Histogram(
		"inventory.stock_check_duration_ms",
		metric.WithDescription("Duration of stock check in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	handler := otelslog.NewHandler("inventory-service")
	logger := slog.New(handler)

	return &InventoryServer{
		stock: map[string]int32{
			"prod-A": 100,
			"prod-B": 50,
			"prod-C": 200,
			"prod-D": 10,
		},
		tracer:             tracer,
		meter:              meter,
		logger:             logger,
		stockChecksTotal:   stockChecksTotal,
		stockCheckDuration: stockCheckDuration,
	}, nil
}

// CheckStock checks whether the requested quantity is available for a product.
func (s *InventoryServer) CheckStock(ctx context.Context, req *inventory.StockRequest) (*inventory.StockResponse, error) {
	ctx, span := s.tracer.Start(ctx, "inventory.CheckStock")
	defer span.End()

	start := time.Now()

	remaining := s.stock[req.ProductId]
	available := remaining >= req.Quantity

	span.SetAttributes(
		attribute.String("product_id", req.ProductId),
		attribute.Int("quantity_requested", int(req.Quantity)),
		attribute.Bool("available", available),
		attribute.Int("remaining", int(remaining)),
	)

	if available {
		span.SetStatus(codes.Ok, "")
	} else {
		span.SetStatus(codes.Error, "insufficient stock")
	}

	s.logger.InfoContext(ctx, "stock check",
		"product_id", req.ProductId,
		"quantity", req.Quantity,
		"available", available,
		"remaining", remaining,
	)

	s.stockChecksTotal.Add(ctx, 1,
		metric.WithAttributes(
			attribute.String("product_id", req.ProductId),
			attribute.Bool("available", available),
		),
	)

	durationMs := float64(time.Since(start).Microseconds()) / 1000.0
	s.stockCheckDuration.Record(ctx, durationMs,
		metric.WithAttributes(
			attribute.String("product_id", req.ProductId),
		),
	)

	return &inventory.StockResponse{
		Available: available,
		Remaining: remaining,
	}, nil
}
