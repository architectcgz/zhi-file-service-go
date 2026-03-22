package httpserver

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"

	"github.com/architectcgz/zhi-file-service-go/internal/platform/config"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/middleware"
	"github.com/architectcgz/zhi-file-service-go/internal/platform/observability"
)

type ReadyFunc func(context.Context) error

type Options struct {
	ServiceName    string
	HTTP           config.HTTPConfig
	Logger         *slog.Logger
	Ready          ReadyFunc
	MetricsHandler http.Handler
	Handler        http.Handler
}

type Server struct {
	logger             *slog.Logger
	srv                *http.Server
	hasBusinessHandler bool
}

func New(options Options) *Server {
	mux := http.NewServeMux()
	mux.HandleFunc("/live", liveHandler)
	mux.HandleFunc("/ready", readyHandler(options.Ready))

	if options.MetricsHandler != nil {
		mux.Handle("/metrics", options.MetricsHandler)
	}
	if options.Handler != nil {
		mux.Handle("/", options.Handler)
	}

	handler := http.Handler(mux)
	handler = middleware.Logging(options.Logger, handler)
	handler = observability.WrapHTTP(options.ServiceName, handler)
	handler = middleware.RequestID(handler)
	return &Server{
		logger:             options.Logger,
		hasBusinessHandler: options.Handler != nil,
		srv: &http.Server{
			Addr:         fmt.Sprintf(":%d", options.HTTP.Port),
			Handler:      handler,
			ReadTimeout:  options.HTTP.ReadTimeout,
			WriteTimeout: options.HTTP.WriteTimeout,
			IdleTimeout:  options.HTTP.IdleTimeout,
		},
	}
}

func (s *Server) Start() error {
	if s.logger != nil {
		s.logger.Info("http_server_start", "addr", s.srv.Addr)
	}
	if err := s.srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
		return err
	}
	return nil
}

func (s *Server) Shutdown(ctx context.Context) error {
	if s.logger != nil {
		s.logger.Info("http_server_shutdown")
	}
	return s.srv.Shutdown(ctx)
}

func (s *Server) Handler() http.Handler {
	return s.srv.Handler
}

func (s *Server) HasBusinessHandler() bool {
	if s == nil {
		return false
	}
	return s.hasBusinessHandler
}

func liveHandler(w http.ResponseWriter, _ *http.Request) {
	w.WriteHeader(http.StatusOK)
	_, _ = w.Write([]byte("ok"))
}

func readyHandler(readyFn ReadyFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if readyFn != nil {
			if err := readyFn(r.Context()); err != nil {
				http.Error(w, http.StatusText(http.StatusServiceUnavailable), http.StatusServiceUnavailable)
				return
			}
		}
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	}
}
