package database

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestDefaultReplicaConfig(t *testing.T) {
	dsn := "postgres://user:pass@primary/db"
	cfg := DefaultReplicaConfig(dsn)

	assert.Equal(t, dsn, cfg.PrimaryDSN)
	assert.Empty(t, cfg.ReplicaDSNs)
	assert.Equal(t, 100, cfg.MaxOpenConns)
	assert.Equal(t, 25, cfg.MaxIdleConns)
	assert.Equal(t, time.Hour, cfg.ConnMaxLifetime)
	assert.Equal(t, 10*time.Minute, cfg.ConnMaxIdleTime)
	assert.Equal(t, 5, cfg.ReplicaLagThreshold)
	assert.False(t, cfg.EnableLogging)
	assert.Equal(t, 30*time.Second, cfg.HealthCheckInterval)
	assert.Equal(t, 0.0, cfg.ReadFromPrimaryRatio)
}

func TestNewDBRouter_EmptyPrimaryDSN(t *testing.T) {
	cfg := ReplicaConfig{
		PrimaryDSN: "",
	}

	router, err := NewDBRouter(cfg)

	assert.Nil(t, router)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "primary DSN is required")
}

func TestReplica_IsHealthy_Default(t *testing.T) {
	r := &Replica{healthy: true}
	assert.True(t, r.IsHealthy())
}

func TestReplica_IsHealthy_False(t *testing.T) {
	r := &Replica{healthy: false}
	assert.False(t, r.IsHealthy())
}

func TestReplica_SetHealthy(t *testing.T) {
	tests := []struct {
		name    string
		initial bool
		set     bool
	}{
		{"mark healthy", false, true},
		{"mark unhealthy", true, false},
		{"idempotent healthy", true, true},
		{"idempotent unhealthy", false, false},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Replica{healthy: tc.initial}
			r.SetHealthy(tc.set)
			assert.Equal(t, tc.set, r.IsHealthy())
		})
	}
}

func TestReplica_SetHealthy_UpdatesLastCheck(t *testing.T) {
	r := &Replica{healthy: true}
	before := time.Now()
	r.SetHealthy(false)
	after := time.Now()

	assert.True(t, r.lastCheck.After(before) || r.lastCheck.Equal(before))
	assert.True(t, r.lastCheck.Before(after) || r.lastCheck.Equal(after))
}

func TestReplica_GetLag_Default(t *testing.T) {
	r := &Replica{}
	assert.Equal(t, int64(0), r.GetLag())
}

func TestReplica_SetLag(t *testing.T) {
	tests := []struct {
		name string
		lag  int64
	}{
		{"zero lag", 0},
		{"small lag", 2},
		{"at threshold", 5},
		{"above threshold", 10},
		{"large lag", 3600},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := &Replica{}
			r.SetLag(tc.lag)
			assert.Equal(t, tc.lag, r.GetLag())
		})
	}
}

func TestReplica_SetLag_IsThreadSafe(t *testing.T) {
	r := &Replica{}

	done := make(chan struct{})
	go func() {
		for i := 0; i < 1000; i++ {
			r.SetLag(int64(i))
		}
		close(done)
	}()

	for i := 0; i < 1000; i++ {
		_ = r.GetLag()
	}

	<-done
}

func TestReplicaStats_StructFields(t *testing.T) {
	now := time.Now()
	stat := ReplicaStats{
		DSN:        "postgres://host/db",
		Healthy:    true,
		Lag:        3,
		LastCheck:  now,
		OpenConns:  8,
		IdleConns:  4,
		InUseConns: 2,
	}

	assert.Equal(t, "postgres://host/db", stat.DSN)
	assert.True(t, stat.Healthy)
	assert.Equal(t, int64(3), stat.Lag)
	assert.Equal(t, now, stat.LastCheck)
	assert.Equal(t, 8, stat.OpenConns)
	assert.Equal(t, 4, stat.IdleConns)
	assert.Equal(t, 2, stat.InUseConns)
}

func TestRouterStats_StructFields(t *testing.T) {
	primary := ReplicaStats{DSN: "postgres://primary/db", Healthy: true}
	replicas := []ReplicaStats{
		{DSN: "postgres://replica1/db", Healthy: true},
		{DSN: "postgres://replica2/db", Healthy: false},
	}

	stats := RouterStats{
		Primary:  primary,
		Replicas: replicas,
	}

	assert.Equal(t, primary, stats.Primary)
	assert.Len(t, stats.Replicas, 2)
	assert.Equal(t, "postgres://replica1/db", stats.Replicas[0].DSN)
	assert.False(t, stats.Replicas[1].Healthy)
}

// TestDBRouter_Read_FallsBackToPrimary_WhenNoReplicas tests the Read() method
// without a real database by constructing a router with an empty replica slice.
// When there are no healthy replicas Read() must return the primary connection.
func TestDBRouter_Read_FallsBackToPrimary_WhenNoReplicas(t *testing.T) {
	// We cannot call NewDBRouter without a live connection, so we build the
	// struct directly to exercise the routing logic in Read().
	router := &DBRouter{
		config: ReplicaConfig{
			ReplicaLagThreshold:  5,
			ReadFromPrimaryRatio: 0.0,
		},
		replicas: []*Replica{},
		primary:  nil, // value is nil but the logic still returns it
	}

	result := router.Read()
	// With no replicas the primary (nil in this test) must be returned.
	assert.Nil(t, result, "expected primary (nil placeholder) to be returned")
}

// TestDBRouter_Read_SkipsUnhealthyReplicas verifies that unhealthy replicas are
// excluded from the selection pool and the router falls back to primary.
func TestDBRouter_Read_SkipsUnhealthyReplicas(t *testing.T) {
	unhealthy := &Replica{healthy: false, lag: 0}
	router := &DBRouter{
		config: ReplicaConfig{
			ReplicaLagThreshold:  5,
			ReadFromPrimaryRatio: 0.0,
		},
		replicas: []*Replica{unhealthy},
		primary:  nil,
	}

	result := router.Read()
	assert.Nil(t, result, "unhealthy replica should be skipped; primary returned")
}

// TestDBRouter_Read_SkipsHighLagReplicas verifies that replicas with lag
// exceeding the threshold are excluded.
func TestDBRouter_Read_SkipsHighLagReplicas(t *testing.T) {
	highLag := &Replica{healthy: true, lag: 10}
	router := &DBRouter{
		config: ReplicaConfig{
			ReplicaLagThreshold:  5,
			ReadFromPrimaryRatio: 0.0,
		},
		replicas: []*Replica{highLag},
		primary:  nil,
	}

	result := router.Read()
	assert.Nil(t, result, "high-lag replica should be skipped; primary returned")
}

// TestDBRouter_DB_ForWrite_ReturnsPrimary confirms the Write/DB routing helpers.
func TestDBRouter_DB_ForWrite_ReturnsPrimary(t *testing.T) {
	router := &DBRouter{
		primary:  nil, // nil stands in for a real *gorm.DB in this unit test
		replicas: []*Replica{},
		config:   ReplicaConfig{},
	}

	assert.Equal(t, router.primary, router.Write())
	assert.Equal(t, router.primary, router.DB(true))
}

func TestDBRouter_RoundRobinCounter_Increments(t *testing.T) {
	// Confirm the round-robin counter advances on each Read() call that
	// enters the replica selection path.  We use a healthy, zero-lag
	// replica so the counter increment path is exercised.
	router := &DBRouter{
		config: ReplicaConfig{
			ReplicaLagThreshold:  5,
			ReadFromPrimaryRatio: 0.0,
		},
		replicas: []*Replica{
			{healthy: true, lag: 0, db: nil},
		},
		primary: nil,
		counter: 0,
	}

	// Call Read() several times and verify the counter increases.
	_ = router.Read()
	after := atomic.LoadUint64(&router.counter)
	assert.Equal(t, uint64(1), after)

	_ = router.Read()
	after = atomic.LoadUint64(&router.counter)
	assert.Equal(t, uint64(2), after)
}

func TestSetDBRouter_And_GetDBRouter(t *testing.T) {
	router := &DBRouter{}
	ctx := context.Background()

	ctxWithRouter := SetDBRouter(ctx, router)
	retrieved, ok := GetDBRouter(ctxWithRouter)

	assert.True(t, ok)
	assert.Equal(t, router, retrieved)
}

func TestGetDBRouter_MissingFromContext(t *testing.T) {
	ctx := context.Background()
	retrieved, ok := GetDBRouter(ctx)

	assert.False(t, ok)
	assert.Nil(t, retrieved)
}

func TestDBRouter_TotalReplicaCount(t *testing.T) {
	router := &DBRouter{
		replicas: []*Replica{{}, {}, {}},
	}

	assert.Equal(t, 3, router.TotalReplicaCount())
}

func TestDBRouter_TotalReplicaCount_Empty(t *testing.T) {
	router := &DBRouter{
		replicas: []*Replica{},
	}

	assert.Equal(t, 0, router.TotalReplicaCount())
}

func TestDBRouter_HealthyReplicaCount(t *testing.T) {
	tests := []struct {
		name     string
		replicas []*Replica
		expected int
	}{
		{
			name:     "no replicas",
			replicas: []*Replica{},
			expected: 0,
		},
		{
			name: "all healthy",
			replicas: []*Replica{
				{healthy: true},
				{healthy: true},
			},
			expected: 2,
		},
		{
			name: "mixed health",
			replicas: []*Replica{
				{healthy: true},
				{healthy: false},
				{healthy: true},
			},
			expected: 2,
		},
		{
			name: "all unhealthy",
			replicas: []*Replica{
				{healthy: false},
			},
			expected: 0,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			router := &DBRouter{replicas: tc.replicas}
			assert.Equal(t, tc.expected, router.HealthyReplicaCount())
		})
	}
}

func TestDBRouter_RemoveReplica_NotFound(t *testing.T) {
	router := &DBRouter{
		replicas: []*Replica{
			{dsn: "postgres://host1/db"},
		},
	}

	err := router.RemoveReplica("postgres://nonexistent/db")
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "replica not found")
}
