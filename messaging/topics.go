package messaging

// Standard Pub/Sub topic names used across the platform.
const (
	TopicAuditEvents        = "tesserix-audit-events"
	TopicNotificationEvents = "tesserix-notification-events"
	TopicSubscriptionEvents = "tesserix-subscription-events"
	TopicTicketEvents       = "tesserix-ticket-events"
)

// Standard event types. Products can define their own in their repos.
const (
	// Auth events
	EventUserLogin        = "auth.user.login"
	EventUserLogout       = "auth.user.logout"
	EventSessionCreated   = "auth.session.created"
	EventSessionRevoked   = "auth.session.revoked"

	// Tenant events
	EventTenantCreated    = "tenant.created"
	EventTenantUpdated    = "tenant.updated"
	EventMemberAdded      = "tenant.member.added"
	EventMemberRemoved    = "tenant.member.removed"

	// Subscription events
	EventPlanChanged      = "subscription.plan.changed"
	EventPaymentReceived  = "subscription.payment.received"
	EventPaymentFailed    = "subscription.payment.failed"

	// Ticket events
	EventTicketCreated       = "ticket.created"
	EventTicketUpdated       = "ticket.updated"
	EventTicketDeleted       = "ticket.deleted"
	EventTicketStatusChanged = "ticket.status_changed"
	EventTicketAssigned      = "ticket.assigned"
	EventTicketUnassigned    = "ticket.unassigned"
	EventTicketCommentAdded  = "ticket.comment_added"
	EventTicketResolved      = "ticket.resolved"
	EventTicketClosed        = "ticket.closed"
	EventTicketEscalated     = "ticket.escalated"
)
