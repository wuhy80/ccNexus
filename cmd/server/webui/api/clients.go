package api

import (
	"net/http"
	"strconv"

	"github.com/lich0821/ccNexus/internal/storage"
)

// handleConnectedClients returns list of connected clients
func (h *Handler) handleConnectedClients(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		WriteError(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	// Get hours parameter, default to 24
	hoursStr := r.URL.Query().Get("hours")
	hours := 24
	if hoursStr != "" {
		if h, err := strconv.Atoi(hoursStr); err == nil && h > 0 {
			hours = h
		}
	}

	clients, err := h.storage.GetConnectedClients(hours)
	if err != nil {
		WriteError(w, http.StatusInternalServerError, "Failed to get connected clients: "+err.Error())
		return
	}

	// Ensure empty slice instead of null in JSON
	if clients == nil {
		clients = []storage.ClientStats{}
	}

	WriteSuccess(w, map[string]interface{}{
		"hoursAgo": hours,
		"count":    len(clients),
		"clients":  clients,
	})
}
