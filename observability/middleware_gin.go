package observability

import (
	"github.com/gin-gonic/gin"
	otelgin "go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
)

// GinMiddleware returns request tracing middleware: one span per request,
// nested spans from any instrumented client (DB, HTTP, messaging) calls
// made while handling it, using the given serviceName as the
// instrumentation/span source name. Requires Setup to have run first so
// a global TracerProvider is registered
func GinMiddleware(serviceName string) gin.HandlerFunc {
	return otelgin.Middleware(serviceName)
}
