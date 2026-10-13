package loxone

import (
	"crypto/tls"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/AES-Concepts-GmbH/loxone2hue/internal/config"
	"github.com/rs/zerolog/log"
)

// httpTarget represents a single HTTP connection to a Miniserver
type httpTarget struct {
	baseURL  string
	user     string
	password string
	sendAll  bool
	name     string
	// verifyTLS selects the client that checks the Miniserver's certificate.
	verifyTLS bool
	// downUntil is set after a failed request. Until it passes, further
	// updates to this target are skipped instead of each waiting for the
	// timeout, so one unreachable Miniserver cannot stall the feedback queue.
	// Guarded by HTTPSender.mu.
	downUntil time.Time
}

// httpBackoff is how long a target is skipped after a failed request.
const httpBackoff = 10 * time.Second

// HTTPSender sends status updates to one or more Loxone Miniservers via HTTP
type HTTPSender struct {
	targets map[string]*httpTarget // key = MiniserverConfig.ID
	// client accepts any certificate (Miniservers use self-signed ones);
	// verifyingClient is used for targets with http_verify_tls set.
	client          *http.Client
	verifyingClient *http.Client
	mu              sync.Mutex
}

// httpTimeout bounds every feedback request. Feedback is fire-and-forget; a
// Miniserver that does not answer within this window is treated as
// unreachable (see sendHTTP).
const httpTimeout = 2 * time.Second

// NewHTTPSender creates a new HTTP sender
func NewHTTPSender() *HTTPSender {
	return &HTTPSender{
		targets: make(map[string]*httpTarget),
		client: &http.Client{
			Timeout: httpTimeout,
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
		},
		verifyingClient: &http.Client{
			Timeout:   httpTimeout,
			Transport: &http.Transport{},
		},
	}
}

// Configure sets up or reconfigures all HTTP targets based on current miniserver config.
func (s *HTTPSender) Configure(miniservers []config.MiniserverConfig) {
	s.mu.Lock()
	defer s.mu.Unlock()

	// Clear existing targets
	s.targets = make(map[string]*httpTarget)

	for _, ms := range miniservers {
		if !ms.HTTPEnabled {
			continue
		}

		baseURL := ms.HTTPURL
		if baseURL == "" {
			if ms.IP == "" {
				continue
			}
			baseURL = fmt.Sprintf("http://%s", ms.IP)
		}

		s.targets[ms.ID] = &httpTarget{
			baseURL:   baseURL,
			user:      ms.HTTPUser,
			password:  ms.HTTPPassword,
			sendAll:   ms.SendAll,
			name:      ms.Name,
			verifyTLS: ms.HTTPVerifyTLS,
		}

		log.Info().Str("id", ms.ID).Str("name", ms.Name).Str("url", baseURL).Bool("verify_tls", ms.HTTPVerifyTLS).
			Msg("HTTP feedback configured for miniserver")
	}
}

// IsEnabled returns whether at least one HTTP target is active
func (s *HTTPSender) IsEnabled() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.targets) > 0
}

// HasSendAll returns true if at least one target has send_all enabled
func (s *HTTPSender) HasSendAll() bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, t := range s.targets {
		if t.sendAll {
			return true
		}
	}
	return false
}

// Close drops all targets and releases the pooled connections.
func (s *HTTPSender) Close() {
	s.mu.Lock()
	s.targets = make(map[string]*httpTarget)
	s.mu.Unlock()

	s.client.CloseIdleConnections()
	s.verifyingClient.CloseIdleConnections()
}

// Send sends a single property update to a specific Miniserver via HTTP.
// Uses Loxone Virtual HTTP Input format: GET /dev/sps/io/<loxone_id>/<value>
func (s *HTTPSender) Send(miniserverID, loxoneID, property string, value interface{}) {
	s.mu.Lock()
	t, ok := s.targets[miniserverID]
	s.mu.Unlock()

	if !ok {
		return
	}

	s.sendHTTP(t, loxoneID, property, value)
}

// SendToAll sends a single property update to all Miniservers that have send_all enabled.
func (s *HTTPSender) SendToAll(loxoneID, property string, value interface{}) {
	s.mu.Lock()
	targets := make([]*httpTarget, 0)
	for _, t := range s.targets {
		if t.sendAll {
			targets = append(targets, t)
		}
	}
	s.mu.Unlock()

	for _, t := range targets {
		s.sendHTTP(t, loxoneID, property, value)
	}
}

// sendHTTP performs the actual HTTP request to the Loxone Miniserver.
// Format: GET /dev/sps/io/<loxone_id>_<property>/<value>
func (s *HTTPSender) sendHTTP(t *httpTarget, loxoneID, property string, value interface{}) {
	viName := fmt.Sprintf("%s_%s", loxoneID, property)

	s.mu.Lock()
	skip := time.Now().Before(t.downUntil)
	s.mu.Unlock()
	if skip {
		log.Debug().Str("miniserver", t.name).Str("vi", viName).Msg("HTTP feedback skipped, miniserver marked unreachable")
		return
	}
	base := strings.TrimRight(t.baseURL, "/")
	// Escape the value as well: sensor payloads such as a contact state are
	// free-form strings and a "/" would otherwise change the request path.
	reqURL := fmt.Sprintf("%s/dev/sps/io/%s/%s", base,
		url.PathEscape(viName), url.PathEscape(fmt.Sprintf("%v", value)))

	req, err := http.NewRequest("GET", reqURL, nil)
	if err != nil {
		log.Warn().Err(err).Str("miniserver", t.name).Str("url", reqURL).Msg("Failed to create HTTP request")
		return
	}

	if t.user != "" {
		req.SetBasicAuth(t.user, t.password)
	}

	client := s.client
	if t.verifyTLS {
		client = s.verifyingClient
	}
	resp, err := client.Do(req)
	if err != nil {
		s.mu.Lock()
		t.downUntil = time.Now().Add(httpBackoff)
		s.mu.Unlock()
		log.Warn().Err(err).Str("miniserver", t.name).Str("url", reqURL).
			Dur("backoff", httpBackoff).Msg("Failed to send HTTP feedback, pausing this miniserver")
		return
	}
	resp.Body.Close()

	log.Info().Str("miniserver", t.name).Str("vi", viName).Interface("value", value).Int("status", resp.StatusCode).Msg("HTTP feedback sent")
}
