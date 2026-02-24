package health

import (
	"context"
	"net/http"
	"os"
	"sync"
	"time"

	"github.com/google/uuid"
	"go.uber.org/zap"
)

// Config holds health check configuration.
type Config struct {
	CheckInterval time.Duration
	ProbeTimeout  time.Duration
	FailThreshold int
}

// EndpointLister returns active agents with endpoints to probe.
type EndpointLister interface {
	ListActiveEndpoints(ctx context.Context) ([]EndpointAgent, error)
}

// StatusUpdater updates an agent's health status.
type StatusUpdater interface {
	UpdateHealthStatus(ctx context.Context, id uuid.UUID, status string, lastSeenAt time.Time) error
}

// EndpointAgent is the minimal data needed for health probes.
type EndpointAgent struct {
	ID       uuid.UUID
	URI      string
	Endpoint string
}

// WebhookDispatchFunc is an optional callback for dispatching health-degraded events.
type WebhookDispatchFunc func(ctx context.Context, eventType string, payload map[string]string)

// MetricsRecordFunc is an optional callback for recording health check results.
type MetricsRecordFunc func(success bool)

// HealthChecker runs periodic endpoint health probes.
type HealthChecker struct {
	lister        EndpointLister
	updater       StatusUpdater
	httpClient    *http.Client
	failCounts    map[uuid.UUID]int
	mu            sync.Mutex
	cfg           Config
	onWebhook     WebhookDispatchFunc
	onMetrics     MetricsRecordFunc
	logger        *zap.Logger
}

// New creates a new HealthChecker.
func New(lister EndpointLister, updater StatusUpdater, cfg Config, logger *zap.Logger) *HealthChecker {
	if cfg.CheckInterval == 0 {
		cfg.CheckInterval = 5 * time.Minute
	}
	if cfg.ProbeTimeout == 0 {
		cfg.ProbeTimeout = 10 * time.Second
	}
	if cfg.FailThreshold == 0 {
		cfg.FailThreshold = 3
	}

	return &HealthChecker{
		lister:     lister,
		updater:    updater,
		httpClient: &http.Client{Timeout: cfg.ProbeTimeout},
		failCounts: make(map[uuid.UUID]int),
		cfg:        cfg,
		logger:     logger,
	}
}

// SetWebhookDispatch configures the webhook dispatch callback.
func (h *HealthChecker) SetWebhookDispatch(fn WebhookDispatchFunc) {
	h.onWebhook = fn
}

// SetMetricsRecord configures the metrics recording callback.
func (h *HealthChecker) SetMetricsRecord(fn MetricsRecordFunc) {
	h.onMetrics = fn
}

// Start runs the health check loop until quit is signalled.
func (h *HealthChecker) Start(quit <-chan os.Signal) {
	ticker := time.NewTicker(h.cfg.CheckInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), h.cfg.CheckInterval-time.Second)
			h.CheckAll(ctx)
			cancel()
		case <-quit:
			return
		}
	}
}

// CheckAll probes all active agent endpoints with bounded concurrency.
func (h *HealthChecker) CheckAll(ctx context.Context) {
	agents, err := h.lister.ListActiveEndpoints(ctx)
	if err != nil {
		h.logger.Error("health: list endpoints", zap.Error(err))
		return
	}

	sem := make(chan struct{}, 10)
	var wg sync.WaitGroup

	for _, a := range agents {
		wg.Add(1)
		go func(agent EndpointAgent) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			success := h.probeEndpoint(ctx, agent.Endpoint)

			if h.onMetrics != nil {
				h.onMetrics(success)
			}

			h.mu.Lock()
			prevCount := h.failCounts[agent.ID]
			if success {
				h.failCounts[agent.ID] = 0
			} else {
				h.failCounts[agent.ID]++
			}
			count := h.failCounts[agent.ID]
			h.mu.Unlock()

			now := time.Now().UTC()

			if success && prevCount >= h.cfg.FailThreshold {
				// Transition: degraded → healthy
				if err := h.updater.UpdateHealthStatus(ctx, agent.ID, "healthy", now); err != nil {
					h.logger.Warn("health: update status", zap.Error(err))
				}
				h.logger.Info("health: recovered", zap.String("uri", agent.URI))
			} else if success {
				if err := h.updater.UpdateHealthStatus(ctx, agent.ID, "healthy", now); err != nil {
					h.logger.Warn("health: update status", zap.Error(err))
				}
			} else if count == h.cfg.FailThreshold {
				// Transition: healthy → degraded (exactly at threshold)
				if err := h.updater.UpdateHealthStatus(ctx, agent.ID, "degraded", now); err != nil {
					h.logger.Warn("health: update status", zap.Error(err))
				}
				h.logger.Warn("health: degraded",
					zap.String("uri", agent.URI),
					zap.Int("fail_count", count),
				)
				if h.onWebhook != nil {
					h.onWebhook(ctx, "agent.health_degraded", map[string]string{
						"agent_id": agent.ID.String(),
						"uri":      agent.URI,
					})
				}
			}
		}(a)
	}

	wg.Wait()
}

// probeEndpoint attempts HEAD then GET, returning true if any 2xx response.
func (h *HealthChecker) probeEndpoint(ctx context.Context, endpoint string) bool {
	// Try HEAD first.
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, endpoint, nil)
	if err != nil {
		return false
	}
	resp, err := h.httpClient.Do(req)
	if err == nil {
		resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			return true
		}
	}

	// Fallback to GET.
	req, err = http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return false
	}
	resp, err = h.httpClient.Do(req)
	if err != nil {
		return false
	}
	resp.Body.Close()
	return resp.StatusCode >= 200 && resp.StatusCode < 300
}
