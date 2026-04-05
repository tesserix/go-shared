package gdpr

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

// Middleware provides GDPR-related middleware for HTTP handlers
type Middleware struct {
	service *Service
}

// NewMiddleware creates a new GDPR middleware instance
func NewMiddleware(service *Service) *Middleware {
	return &Middleware{
		service: service,
	}
}

// RequireConsent creates middleware that checks for active consent
func (m *Middleware) RequireConsent(purpose ConsentPurpose) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "user ID not found in context",
			})
			c.Abort()
			return
		}

		tenantID := c.GetString("tenant_id")
		ctx := ContextWithTenant(c.Request.Context(), tenantID)

		hasConsent, err := m.service.HasActiveConsent(ctx, userID, purpose)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{
				"error":   "consent_check_failed",
				"message": "failed to verify consent status",
			})
			c.Abort()
			return
		}

		if !hasConsent {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   ErrCodeGDPRConsentRequired,
				"message": "consent required for this operation",
				"details": gin.H{
					"purpose": purpose,
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// RequireAnyConsent creates middleware that checks for any of the specified consents
func (m *Middleware) RequireAnyConsent(purposes ...ConsentPurpose) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "user ID not found in context",
			})
			c.Abort()
			return
		}

		tenantID := c.GetString("tenant_id")
		ctx := ContextWithTenant(c.Request.Context(), tenantID)

		for _, purpose := range purposes {
			hasConsent, err := m.service.HasActiveConsent(ctx, userID, purpose)
			if err == nil && hasConsent {
				c.Next()
				return
			}
		}

		c.JSON(http.StatusForbidden, gin.H{
			"error":   ErrCodeGDPRConsentRequired,
			"message": "consent required for this operation",
			"details": gin.H{
				"required_purposes": purposes,
			},
		})
		c.Abort()
	}
}

// RequireAllConsents creates middleware that checks for all specified consents
func (m *Middleware) RequireAllConsents(purposes ...ConsentPurpose) gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.JSON(http.StatusUnauthorized, gin.H{
				"error":   "unauthorized",
				"message": "user ID not found in context",
			})
			c.Abort()
			return
		}

		tenantID := c.GetString("tenant_id")
		ctx := ContextWithTenant(c.Request.Context(), tenantID)

		missingConsents := []ConsentPurpose{}
		for _, purpose := range purposes {
			hasConsent, err := m.service.HasActiveConsent(ctx, userID, purpose)
			if err != nil || !hasConsent {
				missingConsents = append(missingConsents, purpose)
			}
		}

		if len(missingConsents) > 0 {
			c.JSON(http.StatusForbidden, gin.H{
				"error":   ErrCodeGDPRConsentRequired,
				"message": "consent required for this operation",
				"details": gin.H{
					"missing_consents": missingConsents,
				},
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// CheckDeletionPending creates middleware that checks if user deletion is pending
func (m *Middleware) CheckDeletionPending() gin.HandlerFunc {
	return func(c *gin.Context) {
		userID := c.GetString("user_id")
		if userID == "" {
			c.Next()
			return
		}

		tenantID := c.GetString("tenant_id")
		ctx := ContextWithTenant(c.Request.Context(), tenantID)

		requests, err := m.service.ListDeletionRequests(ctx, userID)
		if err != nil {
			c.Next()
			return
		}

		for _, req := range requests {
			if req.Status == RequestStatusPending || req.Status == RequestStatusProcessing {
				c.JSON(http.StatusForbidden, gin.H{
					"error":   ErrCodeGDPRDeletionPending,
					"message": "account is scheduled for deletion",
					"details": gin.H{
						"request_id":    req.ID,
						"scheduled_for": req.ScheduledFor,
					},
				})
				c.Abort()
				return
			}
		}

		c.Next()
	}
}

// AuditRequest creates middleware that logs GDPR-related requests
func (m *Middleware) AuditRequest(action AuditAction, entityType string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Process request first
		c.Next()

		// Then log the action
		userID := c.GetString("user_id")
		tenantID := c.GetString("tenant_id")

		if userID == "" {
			return
		}

		ctx := ContextWithTenant(c.Request.Context(), tenantID)

		m.service.CreateAuditEntry(ctx, GDPRAuditLog{
			TenantID:    tenantID,
			UserID:      userID,
			Action:      action,
			EntityType:  entityType,
			EntityID:    c.Param("id"),
			IPAddress:   c.ClientIP(),
			UserAgent:   c.Request.UserAgent(),
			PerformedBy: userID,
		})
	}
}

// Handler provides HTTP handlers for GDPR operations
type Handler struct {
	service *Service
}

// NewHandler creates a new GDPR handler instance
func NewHandler(service *Service) *Handler {
	return &Handler{
		service: service,
	}
}

// RegisterRoutes registers GDPR routes with a Gin router group
func (h *Handler) RegisterRoutes(rg *gin.RouterGroup) {
	gdpr := rg.Group("/gdpr")
	{
		// Consent management
		consent := gdpr.Group("/consent")
		{
			consent.POST("", h.RecordConsent)
			consent.GET("", h.ListConsents)
			consent.GET("/:purpose", h.GetConsentStatus)
			consent.DELETE("/:purpose", h.WithdrawConsent)
			consent.GET("/:purpose/history", h.GetConsentHistory)
			consent.GET("/summary", h.GetConsentSummary)
		}

		// Data export (Article 20)
		export := gdpr.Group("/export")
		{
			export.POST("", h.RequestDataExport)
			export.GET("/download", h.DownloadDataExport)
		}

		// Right to deletion (Article 17)
		deletion := gdpr.Group("/deletion")
		{
			deletion.POST("", h.RequestDeletion)
			deletion.GET("", h.ListDeletionRequests)
			deletion.GET("/:id", h.GetDeletionRequest)
			deletion.DELETE("/:id", h.CancelDeletionRequest)
			deletion.GET("/:id/verify", h.VerifyDeletion)
		}

		// Audit log
		audit := gdpr.Group("/audit")
		{
			audit.GET("", h.GetAuditLog)
		}
	}
}

// RecordConsent handles consent recording
func (h *Handler) RecordConsent(c *gin.Context) {
	var req ConsentRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.UserID = c.GetString("user_id")
	req.TenantID = c.GetString("tenant_id")
	req.IPAddress = c.ClientIP()
	req.UserAgent = c.Request.UserAgent()

	ctx := ContextWithTenant(c.Request.Context(), req.TenantID)

	consent, err := h.service.RecordConsent(ctx, req)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, consent)
}

// ListConsents handles listing user consents
func (h *Handler) ListConsents(c *gin.Context) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")
	ctx := ContextWithTenant(c.Request.Context(), tenantID)

	consents, err := h.service.ListConsents(ctx, userID)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"consents": consents,
		"count":    len(consents),
	})
}

// GetConsentStatus handles getting consent status for a purpose
func (h *Handler) GetConsentStatus(c *gin.Context) {
	userID := c.GetString("user_id")
	tenantID := c.GetString("tenant_id")
	purpose := ConsentPurpose(c.Param("purpose"))
	ctx := ContextWithTenant(c.Request.Context(), tenantID)

	consent, err := h.service.GetConsentStatus(ctx, userID, purpose)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, consent)
}

// WithdrawConsent handles consent withdrawal
func (h *Handler) WithdrawConsent(c *gin.Context) {
	userID := c.GetString("user_id")
	purpose := ConsentPurpose(c.Param("purpose"))
	ctx := c.Request.Context()

	if err := h.service.WithdrawConsent(ctx, userID, purpose); err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message": "consent withdrawn successfully",
		"purpose": purpose,
	})
}

// GetConsentHistory handles getting consent history
func (h *Handler) GetConsentHistory(c *gin.Context) {
	userID := c.GetString("user_id")
	purpose := ConsentPurpose(c.Param("purpose"))
	ctx := c.Request.Context()

	history, err := h.service.GetConsentHistory(ctx, userID, purpose)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"history": history,
		"count":   len(history),
	})
}

// GetConsentSummary handles getting consent summary
func (h *Handler) GetConsentSummary(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := c.Request.Context()

	summary, err := h.service.GetConsentSummary(ctx, userID)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, summary)
}

// RequestDataExport handles data export requests
func (h *Handler) RequestDataExport(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := c.Request.Context()

	export, err := h.service.ExportUserData(ctx, userID)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, export)
}

// DownloadDataExport handles data export download
func (h *Handler) DownloadDataExport(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := c.Request.Context()

	data, err := h.service.GenerateDataPackage(ctx, userID)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.Header("Content-Disposition", "attachment; filename=gdpr-data-export.zip")
	c.Header("Content-Type", "application/zip")
	c.Data(http.StatusOK, "application/zip", data)
}

// RequestDeletion handles deletion requests
func (h *Handler) RequestDeletion(c *gin.Context) {
	var req DeletionRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "invalid request body"})
		return
	}

	req.UserID = c.GetString("user_id")
	req.TenantID = c.GetString("tenant_id")
	req.IPAddress = c.ClientIP()
	req.UserAgent = c.Request.UserAgent()

	ctx := c.Request.Context()

	deletion, err := h.service.RequestDeletion(ctx, req)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusAccepted, deletion)
}

// ListDeletionRequests handles listing deletion requests
func (h *Handler) ListDeletionRequests(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := c.Request.Context()

	requests, err := h.service.ListDeletionRequests(ctx, userID)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"requests": requests,
		"count":    len(requests),
	})
}

// GetDeletionRequest handles getting a deletion request
func (h *Handler) GetDeletionRequest(c *gin.Context) {
	requestID := c.Param("id")
	ctx := c.Request.Context()

	request, err := h.service.GetDeletionRequest(ctx, requestID)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, request)
}

// CancelDeletionRequest handles canceling a deletion request
func (h *Handler) CancelDeletionRequest(c *gin.Context) {
	requestID := c.Param("id")
	userID := c.GetString("user_id")
	ctx := c.Request.Context()

	if err := h.service.CancelDeletionRequest(ctx, requestID, userID); err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"message":    "deletion request cancelled",
		"request_id": requestID,
	})
}

// VerifyDeletion handles deletion verification
func (h *Handler) VerifyDeletion(c *gin.Context) {
	requestID := c.Param("id")
	ctx := c.Request.Context()

	verification, err := h.service.VerifyDeletion(ctx, requestID)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, verification)
}

// GetAuditLog handles getting audit logs
func (h *Handler) GetAuditLog(c *gin.Context) {
	userID := c.GetString("user_id")
	ctx := c.Request.Context()

	filter := AuditFilter{
		Limit: 100,
	}

	logs, err := h.service.GetAuditLog(ctx, userID, filter)
	if err != nil {
		handleGDPRError(c, err)
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"audit_logs": logs,
		"count":      len(logs),
	})
}

// handleGDPRError handles GDPR errors and returns appropriate HTTP responses
func handleGDPRError(c *gin.Context, err error) {
	if gdprErr := GetGDPRError(err); gdprErr != nil {
		c.JSON(gdprErr.StatusCode, gin.H{
			"error":   gdprErr.Code,
			"message": gdprErr.Message,
			"details": gdprErr.Details,
		})
		return
	}

	// Handle standard errors
	switch err {
	case ErrInvalidUserID:
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrCodeGDPRInvalidRequest, "message": err.Error()})
	case ErrConsentNotFound, ErrRequestNotFound, ErrPolicyNotFound:
		c.JSON(http.StatusNotFound, gin.H{"error": ErrCodeGDPRNotFound, "message": err.Error()})
	case ErrDeletionRequestExists:
		c.JSON(http.StatusConflict, gin.H{"error": ErrCodeGDPRDeletionPending, "message": err.Error()})
	case ErrConsentAlreadyWithdrawn, ErrCannotCancelRequest:
		c.JSON(http.StatusBadRequest, gin.H{"error": ErrCodeGDPRInvalidRequest, "message": err.Error()})
	default:
		c.JSON(http.StatusInternalServerError, gin.H{"error": ErrCodeGDPRInternalError, "message": "internal error"})
	}
}
