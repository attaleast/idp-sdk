package health

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// RegisterRoutes wires the standart k8s probe endpoints onto r:
//
//	GET /healthz - liveness: always 200 once the process is up, on
//								 dependency checks. Point the Deployment's
//								 livenessProbe here.
//	GET /readyz  - readiness: runs the registry, 200 if every check
//								 passes, 503 otherwise with a JSON breakdown. Point
//								 the Deployment's readinessProbe (and Service
//								 endpoint gating) here.
func RegisterRoutes(r gin.IRouter, reg *Registry) {
	r.GET("/healthz", func(ctx *gin.Context) {
		ctx.JSON(http.StatusOK, gin.H{"stauts": "ok"})
	})
	r.GET("/readyz", func(ctx *gin.Context) {
		report := reg.Check(ctx.Request.Context())
		status := http.StatusOK
		if !report.Ready {
			status = http.StatusServiceUnavailable
		}
		ctx.JSON(status, report)
	})
}
