package middleware

import (
	"crypto/rand"
	"encoding/hex"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/propagation"
)

const traceIDHeader = "X-Trace-ID"

// Trace injects an OTel span into the request context and propagates TraceContext headers.
func Trace() gin.HandlerFunc {
	return func(c *gin.Context) {
		// Extract existing trace context from incoming headers (if any)
		prop := otel.GetTextMapPropagator()
		ctx := prop.Extract(c.Request.Context(), propagation.HeaderCarrier(c.Request.Header))

		tracer := otel.Tracer("gateway")
		ctx, span := tracer.Start(ctx, c.FullPath())
		defer span.End()

		// Attach trace ID to response header for client-side correlation
		traceID := span.SpanContext().TraceID().String()
		if traceID == "00000000000000000000000000000000" {
			traceID = newRandID()
		}
		c.Header(traceIDHeader, traceID)
		c.Set("trace_id", traceID)

		// Replace request context
		c.Request = c.Request.WithContext(ctx)
		c.Next()
	}
}

func newRandID() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}