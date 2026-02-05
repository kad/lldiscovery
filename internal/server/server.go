package server

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"time"

	"kad.name/lldiscovery/internal/export"
	"kad.name/lldiscovery/internal/graph"
)

type Server struct {
	addr         string
	graph        *graph.Graph
	logger       *slog.Logger
	showSegments bool
	srv          *http.Server
}

func New(addr string, g *graph.Graph, logger *slog.Logger, showSegments bool) *Server {
	s := &Server{
		addr:         addr,
		graph:        g,
		logger:       logger,
		showSegments: showSegments,
	}

	mux := http.NewServeMux()
	mux.HandleFunc("/graph", s.handleGraph)
	mux.HandleFunc("/graph.dot", s.handleGraphDOT)
	mux.HandleFunc("/graph.nwdiag", s.handleGraphNwdiag)
	mux.HandleFunc("/health", s.handleHealth)

	s.srv = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	return s
}

func (s *Server) Run(ctx context.Context) error {
	errChan := make(chan error, 1)

	go func() {
		s.logger.Info("starting HTTP server", "address", s.addr)
		if err := s.srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			errChan <- err
		}
	}()

	select {
	case err := <-errChan:
		return err
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return s.srv.Shutdown(shutdownCtx)
	}
}

func (s *Server) handleGraph(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes := s.graph.GetNodes()
	edges := s.graph.GetEdges()

	// Build response with full topology information
	response := map[string]interface{}{
		"nodes": nodes,
		"edges": edges,
	}

	// Include segments if enabled
	if s.showSegments {
		segments := s.graph.GetNetworkSegments()
		response["segments"] = segments
	}

	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(response); err != nil {
		s.logger.Error("failed to encode JSON", "error", err)
		http.Error(w, "internal server error", http.StatusInternalServerError)
	}
}

func (s *Server) handleGraphDOT(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes := s.graph.GetNodes()
	edges := s.graph.GetEdges()

	var dot string
	if s.showSegments {
		segments := s.graph.GetNetworkSegments()
		dot = export.GenerateDOTWithSegments(nodes, edges, segments)
	} else {
		dot = export.GenerateDOT(nodes, edges)
	}

	w.Header().Set("Content-Type", "text/vnd.graphviz")
	w.Write([]byte(dot))
}

func (s *Server) handleGraphNwdiag(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	nodes := s.graph.GetNodes()
	segments := s.graph.GetNetworkSegments()

	nwdiag := export.ExportNwdiag(nodes, segments)

	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Write([]byte(nwdiag))
}

func (s *Server) handleHealth(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
	})
}
