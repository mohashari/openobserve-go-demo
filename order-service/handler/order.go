package handler

import (
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/metric"
	"go.opentelemetry.io/otel/trace"

	"github.com/demo/order-service/client"
)

type OrderHandler struct {
	inventoryClient *client.InventoryClient
	tracer          trace.Tracer
	meter           metric.Meter
	logger          *slog.Logger
	ordersTotal     metric.Int64Counter
	orderDuration   metric.Float64Histogram
}

func NewOrderHandler(inventoryClient *client.InventoryClient, logger *slog.Logger) (*OrderHandler, error) {
	tracer := otel.Tracer("order-service")
	meter := otel.Meter("order-service")

	ordersTotal, err := meter.Int64Counter("orders_total",
		metric.WithDescription("Total number of orders processed"),
	)
	if err != nil {
		return nil, err
	}

	orderDuration, err := meter.Float64Histogram("order_processing_duration_ms",
		metric.WithDescription("Duration of order processing in milliseconds"),
		metric.WithUnit("ms"),
	)
	if err != nil {
		return nil, err
	}

	return &OrderHandler{
		inventoryClient: inventoryClient,
		tracer:          tracer,
		meter:           meter,
		logger:          logger,
		ordersTotal:     ordersTotal,
		orderDuration:   orderDuration,
	}, nil
}

type OrderRequest struct {
	ProductID string `json:"product_id"`
	Quantity  int32  `json:"quantity"`
}

type OrderResponse struct {
	Status    string `json:"status"`
	Remaining int32  `json:"remaining,omitempty"`
	Reason    string `json:"reason,omitempty"`
}

func (h *OrderHandler) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	ctx := r.Context()
	ctx, span := h.tracer.Start(ctx, "order.CreateOrder")
	defer span.End()

	start := time.Now()

	var req OrderRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		span.SetStatus(codes.Error, "invalid request")
		span.RecordError(err)
		http.Error(w, "invalid request body", http.StatusBadRequest)
		return
	}

	if req.ProductID == "" || req.Quantity <= 0 {
		validationErr := fmt.Errorf("product_id must be non-empty and quantity must be > 0")
		span.SetStatus(codes.Error, "invalid request")
		span.RecordError(validationErr)
		http.Error(w, validationErr.Error(), http.StatusBadRequest)
		return
	}

	span.SetAttributes(
		attribute.String("product_id", req.ProductID),
		attribute.Int("quantity", int(req.Quantity)),
	)

	stockResp, err := h.inventoryClient.CheckStock(ctx, req.ProductID, req.Quantity)
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		span.RecordError(err)
		h.logger.ErrorContext(ctx, "failed to check stock", "error", err, "product_id", req.ProductID)
		http.Error(w, "inventory service unavailable", http.StatusServiceUnavailable)
		return
	}

	durationMs := float64(time.Since(start).Microseconds()) / 1000.0

	var status string
	var httpStatus int
	var resp OrderResponse

	if stockResp.Available {
		status = "confirmed"
		httpStatus = http.StatusOK
		resp = OrderResponse{
			Status:    "confirmed",
			Remaining: stockResp.Remaining,
		}
	} else {
		status = "rejected"
		httpStatus = http.StatusConflict
		resp = OrderResponse{
			Status:    "rejected",
			Reason:    "insufficient_stock",
			Remaining: stockResp.Remaining,
		}
	}

	h.logger.InfoContext(ctx, "order processed",
		"product_id", req.ProductID,
		"status", status,
		"remaining", stockResp.Remaining,
	)

	h.ordersTotal.Add(ctx, 1, metric.WithAttributes(
		attribute.String("status", status),
	))
	h.orderDuration.Record(ctx, durationMs, metric.WithAttributes(
		attribute.String("status", status),
	))

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(httpStatus)
	if err := json.NewEncoder(w).Encode(resp); err != nil {
		h.logger.ErrorContext(ctx, "failed to encode response", "error", err)
	}
}
