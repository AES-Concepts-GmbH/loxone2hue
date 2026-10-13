package loxone

import (
	"fmt"
	"net"
	"sync"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/rs/zerolog/log"
)

// udpTarget represents a single UDP connection to a Miniserver
type udpTarget struct {
	conn    *net.UDPConn
	sendAll bool
	name    string
}

// UDPSender sends status updates to one or more Loxone Miniservers via UDP
type UDPSender struct {
	targets map[string]*udpTarget // key = MiniserverConfig.ID
	mu      sync.Mutex
}

// NewUDPSender creates a new UDP sender (initially empty)
func NewUDPSender() *UDPSender {
	return &UDPSender{
		targets: make(map[string]*udpTarget),
	}
}

// Configure sets up or reconfigures all UDP targets based on current miniserver config.
// Call this at startup and whenever the config changes.
func (s *UDPSender) Configure(miniservers []config.MiniserverConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Close all existing connections
	for id, t := range s.targets {
		if t.conn != nil {
			t.conn.Close()
		}
		delete(s.targets, id)
	}

	for _, ms := range miniservers {
		if !ms.UDPEnabled || ms.IP == "" || ms.Port == 0 {
			log.Info().Str("id", ms.ID).Str("name", ms.Name).Msg("UDP feedback disabled for miniserver")
			continue
		}

		addr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", ms.IP, ms.Port))
		if err != nil {
			log.Error().Err(err).Str("id", ms.ID).Str("name", ms.Name).Msg("Invalid UDP address for miniserver")
			continue
		}

		conn, err := net.DialUDP("udp", nil, addr)
		if err != nil {
			log.Error().Err(err).Str("id", ms.ID).Str("name", ms.Name).Msg("Failed to create UDP connection for miniserver")
			continue
		}

		s.targets[ms.ID] = &udpTarget{
			conn:    conn,
			sendAll: ms.SendAll,
			name:    ms.Name,
		}

		log.Info().Str("id", ms.ID).Str("name", ms.Name).Str("addr", addr.String()).Msg("UDP feedback configured for miniserver")
	}
}

// IsEnabled returns whether at least one UDP target is active
func (s *UDPSender) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.targets) > 0
}

// IsEnabledFor returns whether a specific miniserver has an active UDP target
func (s *UDPSender) IsEnabledFor(miniserverID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	t, ok := s.targets[miniserverID]
	return ok && t.conn != nil
}

// Send sends a single property update to a specific Miniserver.
// Format: "<loxone_id>/<property>:<value>"
// This is fire-and-forget; errors are logged but not returned.
func (s *UDPSender) Send(miniserverID, loxoneID, property string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	t, ok := s.targets[miniserverID]
	if !ok || t.conn == nil {
		return
	}

	msg := fmt.Sprintf("%s/%s:%v", loxoneID, property, value)

	_, err := t.conn.Write([]byte(msg))
	if err != nil {
		log.Warn().Err(err).Str("miniserver", t.name).Str("msg", msg).Msg("Failed to send UDP feedback")
	} else {
		log.Info().Str("miniserver", t.name).Str("msg", msg).Msg("UDP feedback sent")
	}
}

// SendToAll sends a single property update to all Miniservers that have send_all enabled.
// Used for devices without a specific mapping.
func (s *UDPSender) SendToAll(loxoneID, property string, value interface{}) {
	s.mu.Lock()
	defer s.mu.Unlock()

	msg := fmt.Sprintf("%s/%s:%v", loxoneID, property, value)

	for _, t := range s.targets {
		if !t.sendAll || t.conn == nil {
			continue
		}

		_, err := t.conn.Write([]byte(msg))
		if err != nil {
			log.Warn().Err(err).Str("miniserver", t.name).Str("msg", msg).Msg("Failed to send UDP feedback")
		} else {
			log.Info().Str("miniserver", t.name).Str("msg", msg).Msg("UDP feedback sent")
		}
	}
}

// HasSendAll returns true if at least one target has send_all enabled
func (s *UDPSender) HasSendAll() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.targets {
		if t.sendAll && t.conn != nil {
			return true
		}
	}
	return false
}

// Close closes all UDP connections
func (s *UDPSender) Close() {
	s.mu.Lock()
	defer s.mu.Unlock()

	for id, t := range s.targets {
		if t.conn != nil {
			t.conn.Close()
		}
		delete(s.targets, id)
	}
}
