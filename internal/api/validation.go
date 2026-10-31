package api

import (
	"fmt"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/AES-Concepts-GmbH/loxone2hue/internal/models"
	"net/http"
)

// validateMappings is shared by every mapping write path.
func validateMappings(mappings []models.Mapping, miniservers []config.MiniserverConfig) error {
	ids, targets, servers := map[string]bool{}, map[string]bool{}, map[string]bool{}
	for _, ms := range miniservers {
		servers[ms.ID] = true
	}
	for _, m := range mappings {
		if !isUsableMapping(m) {
			return fmt.Errorf("invalid mapping %q", m.LoxoneID)
		}
		if targets[m.LoxoneID] || (m.ID != "" && ids[m.ID]) {
			return fmt.Errorf("duplicate mapping %q", m.LoxoneID)
		}
		if m.MiniserverID != "" && !servers[m.MiniserverID] {
			return fmt.Errorf("unknown miniserver for mapping %q", m.LoxoneID)
		}
		ids[m.ID], targets[m.LoxoneID] = true, true
	}
	return nil
}

func commitConfig(w http.ResponseWriter, next config.Config) bool {
	if err := config.Commit(next); err != nil {
		errorResponse(w, http.StatusInternalServerError, "configuration not saved: "+err.Error())
		return false
	}
	return true
}
