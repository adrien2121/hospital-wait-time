package main

import (
	"log/slog"
	"net/http"
	"time"

	"github.com/adrien2121/GoProject/internal/handler"
	"github.com/adrien2121/GoProject/internal/logger"
	"github.com/adrien2121/GoProject/internal/repository"
	"github.com/adrien2121/GoProject/internal/service"
)

// buildServices wires repo interfaces into application services.
// Accepts any repository implementation, enabling easy testing with mocks.
func buildServices(
	hospitalRepo repository.HospitalRepository,
	waitTimeRepo repository.WaitTimeRepository,
	signalRepo repository.ExternalSignalRepository,
) (*service.HospitalService, *service.WaitTimeService) {
	return service.NewHospitalService(hospitalRepo),
		service.NewWaitTimeService(waitTimeRepo, hospitalRepo, signalRepo)
}

// buildMux registers all API and health routes on a new ServeMux.
func buildMux(
	hospitalSvc handler.HospitalQuerier,
	waitTimeSvc handler.WaitTimeQuerier,
	pinger handler.Pinger,
	log *slog.Logger,
) *http.ServeMux {
	mux := http.NewServeMux()
	handler.NewHospitalHandler(hospitalSvc, log).RegisterRoutes(mux)
	handler.NewWaitTimeHandler(waitTimeSvc, log).RegisterRoutes(mux)
	handler.NewLivenessHandler(log).RegisterRoutes(mux)
	handler.NewDBReadinessHandler(pinger, log).RegisterRoutes(mux)
	return mux
}

// buildServer sets timeouts to protect against slow-client abuse.
func buildServer(addr string, mux http.Handler) *http.Server {
	return &http.Server{
		Addr:         addr,
		Handler:      mux,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
}

func buildLogger(logLevel string) *slog.Logger { return logger.Build(logLevel) }
