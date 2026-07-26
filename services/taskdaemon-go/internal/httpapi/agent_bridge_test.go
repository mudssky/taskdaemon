package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/stretchr/testify/require"

	"taskdaemon/internal/agentbridge"
	"taskdaemon/internal/config"
	"taskdaemon/internal/data/ent"
	entrun "taskdaemon/internal/data/ent/run"
)

// TestAgentBridgeTriggerUsesBearerNotCookie 验证机器 Bearer 与默认拒绝。
//
// 参数:
//   - t: 测试上下文。
func TestAgentBridgeTriggerUsesBearerNotCookie(t *testing.T) {
	token := "agent-machine-token"
	cfg := config.AgentBridgeConfig{
		Enabled:          true,
		InboundTokenHash: agentbridge.HashToken(token),
		AllowedTaskIDs:   []int{7},
		MaxLoopDepth:     1,
	}
	exit := 0
	tasks := &recordingTaskService{
		triggered: &ent.Run{
			ID:        99,
			Trigger:   entrun.TriggerManual,
			Status:    entrun.StatusSuccess,
			ExitCode:  &exit,
			StartedAt: time.Now().UTC(),
		},
	}
	router := NewRouter(Options{
		Tasks: tasks,
		AgentBridgeConfig: func() config.AgentBridgeConfig {
			return cfg
		},
	})

	// 无 token
	req := httptest.NewRequest(http.MethodPost, "/api/inbound/agent/tasks/7/trigger", nil)
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// 管理员 cookie 不能代替 token
	req = httptest.NewRequest(http.MethodPost, "/api/inbound/agent/tasks/7/trigger", nil)
	req.AddCookie(&http.Cookie{Name: "taskdaemon_session", Value: "admin-session"})
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusUnauthorized, rec.Code)

	// 未允许的任务
	req = httptest.NewRequest(http.MethodPost, "/api/inbound/agent/tasks/8/trigger", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusForbidden, rec.Code)

	// 成功触发
	req = httptest.NewRequest(http.MethodPost, "/api/inbound/agent/tasks/7/trigger", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(agentbridge.HeaderLoopDepth, "0")
	rec = httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusOK, rec.Code)
	require.Equal(t, 7, tasks.triggeredID)
}

// TestAgentBridgeLoopDetected 验证环路深度拒绝。
//
// 参数:
//   - t: 测试上下文。
func TestAgentBridgeLoopDetected(t *testing.T) {
	token := "agent-machine-token"
	cfg := config.AgentBridgeConfig{
		Enabled:          true,
		InboundTokenHash: agentbridge.HashToken(token),
		AllowedTaskIDs:   []int{1},
		MaxLoopDepth:     1,
	}
	router := NewRouter(Options{
		Tasks: &recordingTaskService{},
		AgentBridgeConfig: func() config.AgentBridgeConfig {
			return cfg
		},
	})
	req := httptest.NewRequest(http.MethodPost, "/api/inbound/agent/tasks/1/trigger", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set(agentbridge.HeaderLoopDepth, "1")
	rec := httptest.NewRecorder()
	router.ServeHTTP(rec, req)
	require.Equal(t, http.StatusConflict, rec.Code)
}
