package authz

// Marketplace (MP) permission relations.
// These are OpenFGA relation names defined in the marketplace store model.
// The "MP" prefix is on the Go constant name only — the string value is clean
// because OpenFGA stores are already isolated per product.

// Store-level role relations (direct assignments)
const (
	MPOwner   = "owner"   // Store owner — full unrestricted access
	MPAdmin   = "admin"   // Store admin — full access except ownership transfer
	MPManager = "manager" // Store manager — operations + day-to-day
	MPStaff   = "staff"   // Store staff — basic access
	MPViewer  = "viewer"  // Store viewer — read-only access
)

// Staff & Team management
const (
	MPCanReadStaff       = "can_read_staff"
	MPCanCreateStaff     = "can_create_staff"
	MPCanUpdateStaff     = "can_update_staff"
	MPCanDeleteStaff     = "can_delete_staff"
	MPCanImportStaff     = "can_import_staff"
	MPCanExportStaff     = "can_export_staff"
	MPCanManageStaff     = "can_manage_staff"
	MPCanReadDepartments = "can_read_departments"
	MPCanManageDepts     = "can_manage_departments"
	MPCanReadTeams       = "can_read_teams"
	MPCanManageTeams     = "can_manage_teams"
)

// Roles & Permissions management
const (
	MPCanReadRoles   = "can_read_roles"
	MPCanManageRoles = "can_manage_roles"
	MPCanAssignRoles = "can_assign_roles"
	MPCanReadPerms   = "can_read_permissions"
)

// Orders
const (
	MPCanViewOrders   = "can_view_orders"
	MPCanCreateOrders = "can_create_orders"
	MPCanEditOrders   = "can_edit_orders"
	MPCanCancelOrders = "can_cancel_orders"
	MPCanRefundOrders = "can_refund_orders"
	MPCanFulfillOrders = "can_fulfill_orders"
	MPCanExportOrders = "can_export_orders"
	MPCanManageOrders = "can_manage_orders"
)

// Returns
const (
	MPCanReadReturns    = "can_read_returns"
	MPCanCreateReturns  = "can_create_returns"
	MPCanApproveReturns = "can_approve_returns"
	MPCanRejectReturns  = "can_reject_returns"
	MPCanRefundReturns  = "can_refund_returns"
	MPCanInspectReturns = "can_inspect_returns"
)

// Products & Catalog
const (
	MPCanViewProducts    = "can_view_products"
	MPCanCreateProducts  = "can_create_products"
	MPCanEditProducts    = "can_edit_products"
	MPCanDeleteProducts  = "can_delete_products"
	MPCanPublishProducts = "can_publish_products"
	MPCanManagePricing   = "can_manage_pricing"
	MPCanImportProducts  = "can_import_products"
	MPCanExportProducts  = "can_export_products"
	MPCanManageProducts  = "can_manage_products"
)

// Categories
const (
	MPCanViewCategories   = "can_view_categories"
	MPCanManageCategories = "can_manage_categories"
)

// Inventory
const (
	MPCanViewInventory       = "can_view_inventory"
	MPCanAdjustInventory     = "can_adjust_inventory"
	MPCanViewInventoryHistory = "can_view_inventory_history"
	MPCanManageAlerts        = "can_manage_inventory_alerts"
	MPCanViewTransfers       = "can_view_transfers"
	MPCanManageTransfers     = "can_manage_transfers"
	MPCanViewWarehouses      = "can_view_warehouses"
	MPCanManageWarehouses    = "can_manage_warehouses"
)

// Payments
const (
	MPCanReadPayments       = "can_read_payments"
	MPCanRefundPayments     = "can_refund_payments"
	MPCanReadGateway        = "can_read_gateway"
	MPCanManageGateway      = "can_manage_gateway"
	MPCanManageFees         = "can_manage_fees"
	MPCanViewPaymentMethods = "can_view_payment_methods"
	MPCanEnablePayMethods   = "can_enable_payment_methods"
	MPCanConfigPayMethods   = "can_config_payment_methods"
	MPCanTestPayMethods     = "can_test_payment_methods"
)

// Customers
const (
	MPCanViewCustomers   = "can_view_customers"
	MPCanCreateCustomers = "can_create_customers"
	MPCanEditCustomers   = "can_edit_customers"
	MPCanDeleteCustomers = "can_delete_customers"
	MPCanExportCustomers = "can_export_customers"
	MPCanLockCustomers   = "can_lock_customers"
)

// Vendors (marketplace)
const (
	MPCanReadVendors    = "can_read_vendors"
	MPCanCreateVendors  = "can_create_vendors"
	MPCanUpdateVendors  = "can_update_vendors"
	MPCanApproveVendors = "can_approve_vendors"
	MPCanManageVendors  = "can_manage_vendors"
	MPCanPayoutVendors  = "can_payout_vendors"
)

// Tickets
const (
	MPCanReadTickets     = "can_read_tickets"
	MPCanCreateTickets   = "can_create_tickets"
	MPCanUpdateTickets   = "can_update_tickets"
	MPCanAssignTickets   = "can_assign_tickets"
	MPCanEscalateTickets = "can_escalate_tickets"
	MPCanResolveTickets  = "can_resolve_tickets"
)

// Coupons & Marketing
const (
	MPCanViewCoupons       = "can_view_coupons"
	MPCanManageCoupons     = "can_manage_coupons"
	MPCanViewCampaigns     = "can_view_campaigns"
	MPCanManageCampaigns   = "can_manage_campaigns"
	MPCanSendEmail         = "can_send_email"
	MPCanViewReviews       = "can_view_reviews"
	MPCanModerateReviews   = "can_moderate_reviews"
	MPCanManageBanners     = "can_manage_banners"
	MPCanViewLoyalty       = "can_view_loyalty"
	MPCanManageLoyalty     = "can_manage_loyalty"
	MPCanAdjustPoints      = "can_adjust_loyalty_points"
	MPCanViewCarts         = "can_view_carts"
	MPCanRecoverCarts      = "can_recover_carts"
	MPCanViewSegments      = "can_view_segments"
	MPCanManageSegments    = "can_manage_segments"
)

// Analytics
const (
	MPCanViewDashboard     = "can_view_dashboard"
	MPCanViewReports       = "can_view_reports"
	MPCanViewSalesAnalytics = "can_view_sales_analytics"
	MPCanViewProductStats  = "can_view_product_stats"
	MPCanViewRealtime      = "can_view_realtime"
	MPCanExportReports     = "can_export_reports"
	MPCanViewAnalytics     = "can_view_analytics"
)

// Audit
const (
	MPCanReadAudit  = "can_read_audit"
	MPCanExportAudit = "can_export_audit"
)

// Settings
const (
	MPCanReadSettings   = "can_read_settings"
	MPCanUpdateSettings = "can_update_settings"
	MPCanManageSettings = "can_manage_settings"
)

// Storefronts
const (
	MPCanReadStorefronts   = "can_read_storefronts"
	MPCanCreateStorefronts = "can_create_storefronts"
	MPCanUpdateStorefronts = "can_update_storefronts"
	MPCanDeleteStorefronts = "can_delete_storefronts"
)

// Shipping
const (
	MPCanReadShipping   = "can_read_shipping"
	MPCanCreateShipping = "can_create_shipping"
	MPCanUpdateShipping = "can_update_shipping"
	MPCanManageShipping = "can_manage_shipping"
)

// Notifications
const (
	MPCanReadNotifications   = "can_read_notifications"
	MPCanManageNotifications = "can_manage_notifications"
)

// Approvals
const (
	MPCanReadApprovals    = "can_read_approvals"
	MPCanCreateApprovals  = "can_create_approvals"
	MPCanApprove          = "can_approve"
	MPCanReject           = "can_reject"
	MPCanManageApprovals  = "can_manage_approvals"
)

// Documents
const (
	MPCanReadDocuments   = "can_read_documents"
	MPCanManageDocuments = "can_manage_documents"
	MPCanVerifyDocuments = "can_verify_documents"
)

// Invitations
const (
	MPCanCreateInvitations = "can_create_invitations"
	MPCanReadInvitations   = "can_read_invitations"
	MPCanRevokeInvitations = "can_revoke_invitations"
)

// Delegations
const (
	MPCanReadDelegations   = "can_read_delegations"
	MPCanManageDelegations = "can_manage_delegations"
)

// Gift Cards
const (
	MPCanViewGiftCards  = "can_view_gift_cards"
	MPCanCreateGiftCards = "can_create_gift_cards"
	MPCanEditGiftCards  = "can_edit_gift_cards"
	MPCanDeleteGiftCards = "can_delete_gift_cards"
	MPCanRedeemGiftCards = "can_redeem_gift_cards"
)

// Tax
const (
	MPCanReadTax   = "can_read_tax"
	MPCanManageTax = "can_manage_tax"
)

// Locations
const (
	MPCanReadLocations   = "can_read_locations"
	MPCanManageLocations = "can_manage_locations"
)

// Integrations (SSO/SCIM)
const (
	MPCanViewSSO    = "can_view_sso"
	MPCanManageSSO  = "can_manage_sso"
	MPCanViewSCIM   = "can_view_scim"
	MPCanManageSCIM = "can_manage_scim"
)

// Ads
const (
	MPCanViewAdsCampaigns    = "can_view_ads_campaigns"
	MPCanCreateAdsCampaigns  = "can_create_ads_campaigns"
	MPCanEditAdsCampaigns    = "can_edit_ads_campaigns"
	MPCanDeleteAdsCampaigns  = "can_delete_ads_campaigns"
	MPCanApproveAdsCampaigns = "can_approve_ads_campaigns"
	MPCanPauseAdsCampaigns   = "can_pause_ads_campaigns"
	MPCanViewAdsCreatives    = "can_view_ads_creatives"
	MPCanManageAdsCreatives  = "can_manage_ads_creatives"
	MPCanApproveAdsCreatives = "can_approve_ads_creatives"
	MPCanViewAdsBilling      = "can_view_ads_billing"
	MPCanManageAdsBilling    = "can_manage_ads_billing"
	MPCanRefundAds           = "can_refund_ads"
	MPCanManageAdsTiers      = "can_manage_ads_tiers"
	MPCanViewAdsRevenue      = "can_view_ads_revenue"
	MPCanViewAdsTargeting    = "can_view_ads_targeting"
	MPCanManageAdsTargeting  = "can_manage_ads_targeting"
	MPCanViewAdsAnalytics    = "can_view_ads_analytics"
	MPCanExportAdsAnalytics  = "can_export_ads_analytics"
	MPCanViewAdsPlacements   = "can_view_ads_placements"
	MPCanManageAdsPlacements = "can_manage_ads_placements"
)

// Marketplace Connectors
const (
	MPCanReadConnectors   = "can_read_connectors"
	MPCanCreateConnectors = "can_create_connectors"
	MPCanUpdateConnectors = "can_update_connectors"
	MPCanDeleteConnectors = "can_delete_connectors"
	MPCanManageConnectors = "can_manage_connectors"
)

// Content
const (
	MPCanViewContent   = "can_view_content"
	MPCanCreateContent = "can_create_content"
	MPCanEditContent   = "can_edit_content"
	MPCanDeleteContent = "can_delete_content"
)
