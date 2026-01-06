package service

import (
	"encoding/json"

	"github.com/lich0821/ccNexus/internal/storage"
)

// ClientService handles connected clients operations
type ClientService struct {
	storage *storage.SQLiteStorage
}

// NewClientService creates a new client service
func NewClientService(st *storage.SQLiteStorage) *ClientService {
	return &ClientService{storage: st}
}

// GetConnectedClients returns clients that have made requests recently
func (c *ClientService) GetConnectedClients(hoursAgo int) string {
	if hoursAgo <= 0 {
		hoursAgo = 24 // Default to 24 hours
	}

	clients, err := c.storage.GetConnectedClients(hoursAgo)
	if err != nil {
		result := map[string]interface{}{
			"success": false,
			"message": err.Error(),
			"clients": []interface{}{},
		}
		data, _ := json.Marshal(result)
		return string(data)
	}

	// Ensure empty slice instead of null in JSON
	if clients == nil {
		clients = []storage.ClientStats{}
	}

	result := map[string]interface{}{
		"success":  true,
		"hoursAgo": hoursAgo,
		"count":    len(clients),
		"clients":  clients,
	}
	data, _ := json.Marshal(result)
	return string(data)
}
