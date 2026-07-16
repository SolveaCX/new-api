package model

const (
	StatusOperational = "operational"
	StatusDegraded    = "degraded"
	StatusOutage      = "outage"
	StatusUnknown     = "unknown"
	StatusMaintenance = "maintenance"

	StatusComponentKindRouter = "router"
	StatusComponentKindModel  = "model"
	StatusLifecycleActive     = "active"
	StatusLifecycleRetired    = "retired"

	StatusGranularityFiveMinutes = "5m"
	StatusGranularityHour        = "hour"
	StatusGranularityDay         = "day"

	StatusIncidentKindIncident    = "incident"
	StatusIncidentKindMaintenance = "maintenance"

	StatusSubscriberKindEmail    = "email"
	StatusSubscriberKindWebhook  = "webhook"
	StatusSubscriberPending      = "pending"
	StatusSubscriberActive       = "active"
	StatusSubscriberSuspended    = "suspended"
	StatusSubscriberUnsubscribed = "unsubscribed"

	StatusDestinationEmail   = "email"
	StatusDestinationWebhook = "webhook"
	StatusDestinationDiscord = "discord"
	StatusDeliveryPending    = "pending"
	StatusDeliveryProcessing = "processing"
	StatusDeliveryDelivered  = "delivered"
	StatusDeliveryDead       = "dead"
)

type StatusComponent struct {
	ID                         int64  `json:"id" gorm:"primaryKey"`
	ComponentKey               string `json:"component_key" gorm:"size:191;not null;uniqueIndex"`
	Slug                       string `json:"slug" gorm:"size:191;not null;uniqueIndex"`
	Kind                       string `json:"kind" gorm:"size:32;not null;index"`
	ModelName                  string `json:"model_name,omitempty" gorm:"size:191;index"`
	DisplayName                string `json:"display_name" gorm:"size:191;not null"`
	Capability                 string `json:"capability,omitempty" gorm:"size:64;index"`
	Lifecycle                  string `json:"lifecycle" gorm:"size:32;not null;default:active;index"`
	ObservedStatus             string `json:"observed_status" gorm:"size:32;not null;default:unknown;index"`
	EffectiveStatus            string `json:"effective_status" gorm:"size:32;not null;default:unknown;index"`
	StatusSource               string `json:"status_source" gorm:"size:32;not null;default:observed"`
	LastEvidenceAt             int64  `json:"last_evidence_at" gorm:"not null;default:0"`
	LastTrustworthyUpdateAt    int64  `json:"last_trustworthy_update_at" gorm:"not null;default:0"`
	LastEvaluatedAt            int64  `json:"last_evaluated_at" gorm:"not null;default:0;index"`
	CoverageMicros             int64  `json:"coverage_micros" gorm:"not null;default:0"`
	ConsecutiveProbeFailures   int64  `json:"-" gorm:"not null;default:0"`
	ConsecutiveProbeSuccesses  int64  `json:"-" gorm:"not null;default:0"`
	ConsecutiveTrafficRecovery int64  `json:"-" gorm:"not null;default:0"`
	OverrideStatus             string `json:"override_status,omitempty" gorm:"size:32;not null;default:''"`
	OverrideReason             string `json:"override_reason,omitempty" gorm:"type:text"`
	OverrideExpiresAt          int64  `json:"override_expires_at,omitempty" gorm:"not null;default:0;index"`
	OverrideBy                 int    `json:"override_by,omitempty" gorm:"not null;default:0"`
	OverrideCreatedAt          int64  `json:"override_created_at,omitempty" gorm:"not null;default:0"`
	Version                    int64  `json:"version" gorm:"not null;default:1"`
	CreatedAt                  int64  `json:"created_at" gorm:"not null;default:0"`
	UpdatedAt                  int64  `json:"updated_at" gorm:"not null;default:0"`
}

type StatusPeriod struct {
	ID                     int64  `json:"id" gorm:"primaryKey"`
	ComponentID            int64  `json:"component_id" gorm:"not null;uniqueIndex:idx_status_period,priority:1;index"`
	Granularity            string `json:"granularity" gorm:"size:16;not null;uniqueIndex:idx_status_period,priority:2"`
	PeriodStart            int64  `json:"period_start" gorm:"not null;uniqueIndex:idx_status_period,priority:3;index"`
	ScoreSumMicros         int64  `json:"score_sum_micros" gorm:"not null;default:0"`
	KnownBucketCount       int64  `json:"known_bucket_count" gorm:"not null;default:0"`
	UnknownBucketCount     int64  `json:"unknown_bucket_count" gorm:"not null;default:0"`
	MaintenanceBucketCount int64  `json:"maintenance_bucket_count" gorm:"not null;default:0"`
	WorstStatus            string `json:"worst_status" gorm:"size:32;not null;default:unknown"`
	EligibleCount          int64  `json:"-" gorm:"not null;default:0"`
	SuccessCount           int64  `json:"-" gorm:"not null;default:0"`
	ProbeSuccessCount      int64  `json:"-" gorm:"not null;default:0"`
	ProbeFailureCount      int64  `json:"-" gorm:"not null;default:0"`
	LatencySumMs           int64  `json:"-" gorm:"not null;default:0"`
	LatencyCount           int64  `json:"-" gorm:"not null;default:0"`
	TtftSumMs              int64  `json:"-" gorm:"not null;default:0"`
	TtftCount              int64  `json:"-" gorm:"not null;default:0"`
	CreatedAt              int64  `json:"created_at" gorm:"not null;default:0"`
	UpdatedAt              int64  `json:"updated_at" gorm:"not null;default:0"`
}

type StatusProbeResult struct {
	ID              int64  `json:"id" gorm:"primaryKey"`
	ComponentID     int64  `json:"component_id" gorm:"not null;index"`
	Success         bool   `json:"success" gorm:"not null;default:false"`
	MonitoringFault bool   `json:"monitoring_fault" gorm:"not null;default:false"`
	DiagnosticType  string `json:"diagnostic_type" gorm:"size:64;not null;default:''"`
	TargetRef       string `json:"-" gorm:"size:191;not null;default:''"`
	LatencyMs       int64  `json:"latency_ms" gorm:"not null;default:0"`
	FencingToken    int64  `json:"-" gorm:"not null;default:0"`
	CreatedAt       int64  `json:"created_at" gorm:"not null;index"`
}

type StatusIncident struct {
	ID               int64  `json:"id" gorm:"primaryKey"`
	PublicID         string `json:"public_id" gorm:"size:64;not null;uniqueIndex"`
	Kind             string `json:"kind" gorm:"size:32;not null;index"`
	Title            string `json:"title" gorm:"size:191;not null;default:''"`
	Impact           string `json:"impact" gorm:"size:32;not null;default:none"`
	Status           string `json:"status" gorm:"size:32;not null;default:draft;index"`
	Visibility       string `json:"visibility" gorm:"size:32;not null;default:private;index"`
	AutomationMode   string `json:"automation_mode" gorm:"size:32;not null;default:automatic"`
	IdempotencyKey   string `json:"-" gorm:"size:191;not null;uniqueIndex"`
	ScheduledStartAt int64  `json:"scheduled_start_at,omitempty" gorm:"not null;default:0;index"`
	ScheduledEndAt   int64  `json:"scheduled_end_at,omitempty" gorm:"not null;default:0;index"`
	StartedAt        int64  `json:"started_at,omitempty" gorm:"not null;default:0"`
	ResolvedAt       int64  `json:"resolved_at,omitempty" gorm:"not null;default:0"`
	Version          int64  `json:"version" gorm:"not null;default:1"`
	CreatedBy        int    `json:"created_by,omitempty" gorm:"not null;default:0"`
	CreatedAt        int64  `json:"created_at" gorm:"not null;default:0"`
	UpdatedAt        int64  `json:"updated_at" gorm:"not null;default:0"`
}

type StatusIncidentUpdate struct {
	ID          int64  `json:"id" gorm:"primaryKey"`
	IncidentID  int64  `json:"incident_id" gorm:"not null;index"`
	EventID     string `json:"event_id" gorm:"size:64;not null;uniqueIndex"`
	State       string `json:"state" gorm:"size:32;not null"`
	Body        string `json:"body" gorm:"type:text;not null"`
	Published   bool   `json:"published" gorm:"not null;default:false;index"`
	PublishedAt int64  `json:"published_at,omitempty" gorm:"not null;default:0;index"`
	ActorID     int    `json:"actor_id,omitempty" gorm:"not null;default:0"`
	CreatedAt   int64  `json:"created_at" gorm:"not null;default:0"`
}

type StatusIncidentComponent struct {
	ID          int64 `json:"id" gorm:"primaryKey"`
	IncidentID  int64 `json:"incident_id" gorm:"not null;uniqueIndex:idx_status_incident_component,priority:1;index"`
	ComponentID int64 `json:"component_id" gorm:"not null;uniqueIndex:idx_status_incident_component,priority:2;index"`
}

type StatusSubscriber struct {
	ID                     int64  `json:"id" gorm:"primaryKey"`
	Kind                   string `json:"kind" gorm:"size:32;not null;index"`
	IdentityHash           string `json:"-" gorm:"size:64;not null;uniqueIndex"`
	DisplayAddress         string `json:"display_address,omitempty" gorm:"size:191;not null;default:''"`
	EncryptedEndpoint      string `json:"-" gorm:"type:text"`
	EncryptedSigningSecret string `json:"-" gorm:"type:text"`
	Status                 string `json:"status" gorm:"size:32;not null;index"`
	VerificationTokenHash  string `json:"-" gorm:"size:64;not null;default:'';index"`
	VerificationExpiresAt  int64  `json:"-" gorm:"not null;default:0"`
	ManageTokenHash        string `json:"-" gorm:"size:64;not null;default:'';index"`
	FailureCount           int64  `json:"failure_count" gorm:"not null;default:0"`
	SuspendedAt            int64  `json:"suspended_at,omitempty" gorm:"not null;default:0"`
	CreatedAt              int64  `json:"created_at" gorm:"not null;default:0"`
	UpdatedAt              int64  `json:"updated_at" gorm:"not null;default:0"`
}

type StatusSubscriberComponent struct {
	ID           int64 `json:"id" gorm:"primaryKey"`
	SubscriberID int64 `json:"subscriber_id" gorm:"not null;uniqueIndex:idx_status_subscriber_component,priority:1;index"`
	ComponentID  int64 `json:"component_id" gorm:"not null;uniqueIndex:idx_status_subscriber_component,priority:2;index"`
}

type StatusDeliveryOutbox struct {
	ID                int64  `json:"id" gorm:"primaryKey"`
	PublishedUpdateID int64  `json:"published_update_id" gorm:"not null;uniqueIndex:idx_status_delivery_destination,priority:1;index"`
	DestinationType   string `json:"destination_type" gorm:"size:32;not null;uniqueIndex:idx_status_delivery_destination,priority:2;index"`
	DestinationID     int64  `json:"destination_id" gorm:"not null;uniqueIndex:idx_status_delivery_destination,priority:3;index"`
	EventID           string `json:"event_id" gorm:"size:64;not null;uniqueIndex"`
	Payload           string `json:"-" gorm:"type:text;not null"`
	Status            string `json:"status" gorm:"size:32;not null;index"`
	LockToken         string `json:"-" gorm:"size:64;not null;default:''"`
	LockedUntil       int64  `json:"locked_until,omitempty" gorm:"not null;default:0;index"`
	Attempts          int64  `json:"attempts" gorm:"not null;default:0"`
	NextAttemptAt     int64  `json:"next_attempt_at" gorm:"not null;default:0;index"`
	LastError         string `json:"last_error,omitempty" gorm:"size:191;not null;default:''"`
	Version           int64  `json:"version" gorm:"not null;default:1"`
	CreatedAt         int64  `json:"created_at" gorm:"not null;default:0"`
	UpdatedAt         int64  `json:"updated_at" gorm:"not null;default:0"`
}

type StatusJobLease struct {
	Name         string `json:"name" gorm:"size:191;primaryKey"`
	Holder       string `json:"holder" gorm:"size:191;not null"`
	ExpiresAt    int64  `json:"expires_at" gorm:"not null;index"`
	FencingToken int64  `json:"fencing_token" gorm:"not null;default:1"`
	UpdatedAt    int64  `json:"updated_at" gorm:"not null;default:0"`
}

type StatusAuditEvent struct {
	ID         int64  `json:"id" gorm:"primaryKey"`
	ActorID    int    `json:"actor_id" gorm:"not null;default:0;index"`
	ActorType  string `json:"actor_type" gorm:"size:32;not null;index"`
	Action     string `json:"action" gorm:"size:64;not null;index"`
	ObjectType string `json:"object_type" gorm:"size:64;not null;index"`
	ObjectID   string `json:"object_id" gorm:"size:191;not null;index"`
	BeforeJSON string `json:"-" gorm:"type:text"`
	AfterJSON  string `json:"-" gorm:"type:text"`
	Reason     string `json:"reason,omitempty" gorm:"type:text"`
	CreatedAt  int64  `json:"created_at" gorm:"not null;index"`
}

type StatusSetting struct {
	Key       string `json:"key" gorm:"size:191;primaryKey"`
	Value     string `json:"value" gorm:"type:text"`
	Sensitive bool   `json:"sensitive" gorm:"not null;default:false"`
	Version   int64  `json:"version" gorm:"not null;default:1"`
	UpdatedBy int    `json:"updated_by" gorm:"not null;default:0"`
	UpdatedAt int64  `json:"updated_at" gorm:"not null;default:0"`
}

func StatusCenterModels() []any {
	return []any{
		&StatusComponent{},
		&StatusPeriod{},
		&StatusProbeResult{},
		&StatusIncident{},
		&StatusIncidentUpdate{},
		&StatusIncidentComponent{},
		&StatusSubscriber{},
		&StatusSubscriberComponent{},
		&StatusDeliveryOutbox{},
		&StatusJobLease{},
		&StatusAuditEvent{},
		&StatusSetting{},
	}
}
