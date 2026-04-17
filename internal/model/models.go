package model

import (
	"time"

	"github.com/google/uuid"
)

type User struct {
	ID           uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Username     string     `gorm:"size:50;uniqueIndex;not null"`
	PasswordHash string     `gorm:"size:255;not null"`
	Email        *string    `gorm:"size:100"`
	Phone        *string    `gorm:"size:20"`
	RealName     *string    `gorm:"size:50"`
	AvatarURL    *string    `gorm:"size:500"`
	Role         string     `gorm:"size:20;not null;index"`
	Status       string     `gorm:"size:20;default:active;index"`
	SchoolID     *uuid.UUID `gorm:"type:uuid;index"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	LastLoginAt  *time.Time
	Metadata     map[string]any `gorm:"type:jsonb;serializer:json"`
	School       *School
	UserRoles    []UserRole
}

func (User) TableName() string {
	return "users"
}

type School struct {
	ID            uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name          string    `gorm:"size:100;not null"`
	Code          *string   `gorm:"size:50;uniqueIndex"`
	Province      *string   `gorm:"size:50"`
	City          *string   `gorm:"size:50"`
	District      *string   `gorm:"size:50"`
	Address       *string   `gorm:"size:255"`
	ContactPerson *string   `gorm:"size:50"`
	ContactPhone  *string   `gorm:"size:20"`
	ContactEmail  *string   `gorm:"size:100"`
	Status        string    `gorm:"size:20;default:active;index"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
	Metadata      map[string]any `gorm:"type:jsonb;serializer:json"`
}

func (School) TableName() string {
	return "schools"
}

type Role struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name         string    `gorm:"size:50;uniqueIndex;not null"`
	Code         string    `gorm:"size:50;uniqueIndex;not null"`
	Description  *string
	Permissions  []string `gorm:"type:jsonb;serializer:json"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	ScopeType    string `gorm:"size:20;default:school;not null"`
	IsBuiltin    bool   `gorm:"default:true;not null"`
	IsSuperAdmin bool   `gorm:"default:false;not null"`
	Status       string `gorm:"size:20;default:active;not null"`
}

func (Role) TableName() string {
	return "roles"
}

type UserRole struct {
	ID        uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_user_role_unique"`
	RoleID    uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_user_role_unique"`
	CreatedAt time.Time
	Status    string     `gorm:"size:20;default:active;not null"`
	GrantedBy *uuid.UUID `gorm:"type:uuid"`
	StartsAt  *time.Time
	EndsAt    *time.Time
	UpdatedAt time.Time
	Role      Role
}

func (UserRole) TableName() string {
	return "user_roles"
}

type Permission struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Code         string    `gorm:"size:100;uniqueIndex;not null"`
	ResourceCode string    `gorm:"size:50;not null;uniqueIndex:idx_permission_resource_action"`
	ActionCode   string    `gorm:"size:50;not null;uniqueIndex:idx_permission_resource_action"`
	Name         string    `gorm:"size:100;not null"`
	Description  *string
	Module       *string `gorm:"size:50"`
	IsBuiltin    bool    `gorm:"default:true;not null"`
	Status       string  `gorm:"size:20;default:active;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
}

func (Permission) TableName() string {
	return "permissions"
}

type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey"`
	CreatedAt    time.Time
	CreatedBy    *uuid.UUID `gorm:"type:uuid"`
}

func (RolePermission) TableName() string {
	return "role_permissions"
}

type AuditLog struct {
	ID             uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	UserID         *uuid.UUID `gorm:"type:uuid;index"`
	Action         string     `gorm:"size:100;not null;index"`
	ResourceType   *string    `gorm:"size:50;index"`
	ResourceID     *uuid.UUID `gorm:"type:uuid"`
	RequestMethod  *string    `gorm:"size:10"`
	RequestPath    *string    `gorm:"size:500"`
	ResponseStatus *int
	ErrorMessage   *string
	RequestBody    map[string]any `gorm:"type:jsonb;serializer:json"`
	CreatedAt      time.Time
}

func (AuditLog) TableName() string {
	return "audit_logs"
}

type Project struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name            string    `gorm:"size:100;not null"`
	Description     *string
	SchoolID        *uuid.UUID `gorm:"type:uuid;index"`
	CreatorID       *uuid.UUID `gorm:"type:uuid;index"`
	Status          string     `gorm:"size:20;default:draft;index"`
	Deadline        *time.Time
	StartDate       *time.Time
	EndDate         *time.Time
	Tags            StringArray `gorm:"type:text[]"`
	ExperimentType  *string     `gorm:"size:50"`
	GradeLevel      *string     `gorm:"size:20"`
	Subject         *string     `gorm:"size:50"`
	TotalVideos     int         `gorm:"default:0"`
	CompletedVideos int         `gorm:"default:0"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Metadata        map[string]any `gorm:"type:jsonb;serializer:json"`
	School          *School
	Creator         *User  `gorm:"foreignKey:CreatorID"`
	Tasks           []Task `gorm:"foreignKey:ProjectID"`
}

func (Project) TableName() string {
	return "projects"
}

type Task struct {
	ID              uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProjectID       uuid.UUID `gorm:"type:uuid;not null;index;uniqueIndex:idx_task_project_name"`
	Name            string    `gorm:"size:100;not null;uniqueIndex:idx_task_project_name"`
	Description     *string
	RubricID        uuid.UUID  `gorm:"type:uuid;not null;index"`
	CreatorID       *uuid.UUID `gorm:"type:uuid;index"`
	Status          string     `gorm:"size:20;default:draft;index"`
	StartDate       *time.Time
	Deadline        *time.Time
	EndDate         *time.Time
	TotalVideos     int `gorm:"default:0"`
	CompletedVideos int `gorm:"default:0"`
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Metadata        map[string]any       `gorm:"type:jsonb;serializer:json"`
	Project         *Project             `gorm:"foreignKey:ProjectID"`
	Rubric          *ScoringRubric       `gorm:"foreignKey:RubricID"`
	Creator         *User                `gorm:"foreignKey:CreatorID"`
	AnalysisReports []TaskAnalysisReport `gorm:"foreignKey:TaskID"`
}

func (Task) TableName() string {
	return "tasks"
}

type TaskAnalysisReport struct {
	ID              uuid.UUID      `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TaskID          uuid.UUID      `gorm:"type:uuid;not null;index"`
	Status          string         `gorm:"size:20;not null;index"`
	ReportFormat    string         `gorm:"size:20;not null;default:pdf"`
	FileName        *string        `gorm:"size:255"`
	StoragePath     *string        `gorm:"size:1000"`
	PublicURL       *string        `gorm:"size:1000"`
	TemplateVersion *string        `gorm:"size:50"`
	SnapshotData    map[string]any `gorm:"type:jsonb;serializer:json"`
	RequestedBy     *uuid.UUID     `gorm:"type:uuid;index"`
	StartedAt       *time.Time
	GeneratedAt     *time.Time
	ErrorMessage    *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
	Task            *Task `gorm:"foreignKey:TaskID"`
	Requester       *User `gorm:"foreignKey:RequestedBy"`
}

func (TaskAnalysisReport) TableName() string {
	return "task_analysis_reports"
}

type RubricSubItem struct {
	ID                 string  `json:"id"`
	Requirement        string  `json:"requirement"`
	Score              float64 `json:"score"`
	FullScoreStandard  string  `json:"fullScoreStandard"`
	DeductionItems     string  `json:"deductionItems"`
	DangerousOperation *string `json:"dangerousOperation,omitempty"`
}

type RubricItem struct {
	ID       string          `json:"id"`
	Name     string          `json:"name"`
	Score    float64         `json:"score"`
	SubItems []RubricSubItem `json:"subItems"`
}

type ScoringRubric struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	Name         string    `gorm:"size:100;not null"`
	Description  *string
	TotalScore   int          `gorm:"not null;default:100"`
	TemplateType *string      `gorm:"size:50"`
	SchoolID     *uuid.UUID   `gorm:"type:uuid;index"`
	CreatorID    *uuid.UUID   `gorm:"type:uuid;index"`
	IsTemplate   bool         `gorm:"default:false;index"`
	IsPublic     bool         `gorm:"default:false"`
	Items        []RubricItem `gorm:"type:jsonb;serializer:json;not null"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	School       *School
	Creator      *User `gorm:"foreignKey:CreatorID"`
}

func (ScoringRubric) TableName() string {
	return "scoring_rubrics"
}

type Video struct {
	ID                uuid.UUID  `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	ProjectID         uuid.UUID  `gorm:"type:uuid;not null;index"`
	TaskID            *uuid.UUID `gorm:"type:uuid;index"`
	SchoolID          *uuid.UUID `gorm:"type:uuid;index"`
	StudentID         *uuid.UUID `gorm:"type:uuid;index"`
	ScorerID          *uuid.UUID `gorm:"type:uuid;index"`
	StudentName       string     `gorm:"size:100;not null"`
	StudentNumber     string     `gorm:"size:50;not null;index"`
	Filename          string     `gorm:"size:255;not null"`
	OriginalFilename  *string    `gorm:"size:255"`
	FileSize          int64      `gorm:"not null"`
	Duration          *int
	Resolution        *string  `gorm:"size:20"`
	Format            *string  `gorm:"size:20"`
	StorageURL        *string  `gorm:"size:1000"`
	StoragePath       *string  `gorm:"size:1000"`
	UploadID          *string  `gorm:"size:255;index"`
	Status            string   `gorm:"size:20;default:uploading;index"`
	TranscodeStatus   *string  `gorm:"size:20"`
	UploadProgress    int      `gorm:"default:0"`
	ThumbnailURL      *string  `gorm:"size:1000"`
	EvaluationStatus  string   `gorm:"size:20;default:pending;index"`
	AIStatus          string   `gorm:"size:20;default:pending;index"`
	ManualStatus      string   `gorm:"size:20;default:pending;index"`
	AIScore           *float64 `gorm:"type:decimal(5,2)"`
	ManualScore       *float64 `gorm:"type:decimal(5,2)"`
	AssignedAt        *time.Time
	CompletedAt       *time.Time
	CreatorID         *uuid.UUID `gorm:"type:uuid;index"`
	UploadedAt        *time.Time
	CreatedAt         time.Time
	UpdatedAt         time.Time
	Metadata          map[string]any `gorm:"type:jsonb;serializer:json"`
	Project           *Project       `gorm:"foreignKey:ProjectID"`
	Task              *Task          `gorm:"foreignKey:TaskID"`
	School            *School
	Scorer            *User              `gorm:"foreignKey:ScorerID"`
	Creator           *User              `gorm:"foreignKey:CreatorID"`
	AIEvaluations     []AIEvaluation     `gorm:"foreignKey:VideoID"`
	ManualEvaluations []ManualEvaluation `gorm:"foreignKey:VideoID"`
}

func (Video) TableName() string {
	return "videos"
}

type AIEvaluation struct {
	ID           uuid.UUID `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TaskID       uuid.UUID `gorm:"type:uuid;not null;index"`
	VideoID      uuid.UUID `gorm:"type:uuid;not null;index"`
	JobID        *string   `gorm:"column:job_id;size:100;index"`
	ModelVersion *string   `gorm:"size:50"`
	TotalScore   *float64  `gorm:"type:decimal(5,2)"`
	Status       string    `gorm:"size:20;default:processing;not null;index"`
	StartedAt    *time.Time
	CompletedAt  *time.Time
	ErrorMessage *string
	ResultData   map[string]any `gorm:"type:jsonb;serializer:json"`
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Task         *Task  `gorm:"foreignKey:TaskID"`
	Video        *Video `gorm:"foreignKey:VideoID"`
}

func (AIEvaluation) TableName() string {
	return "ai_evaluations"
}

type ManualEvaluation struct {
	ID           uuid.UUID        `gorm:"type:uuid;default:gen_random_uuid();primaryKey"`
	TaskID       uuid.UUID        `gorm:"type:uuid;not null;index"`
	VideoID      uuid.UUID        `gorm:"type:uuid;not null;index"`
	ScorerID     uuid.UUID        `gorm:"type:uuid;not null;index"`
	RubricID     *uuid.UUID       `gorm:"type:uuid;index"`
	TotalScore   *float64         `gorm:"type:decimal(5,2)"`
	ScoreDetails []map[string]any `gorm:"type:jsonb;serializer:json"`
	Comments     *string
	Status       string `gorm:"size:20;default:in_progress;not null;index"`
	StartedAt    *time.Time
	SubmittedAt  *time.Time
	TimeSpent    *int
	CreatedAt    time.Time
	UpdatedAt    time.Time
	Task         *Task          `gorm:"foreignKey:TaskID"`
	Video        *Video         `gorm:"foreignKey:VideoID"`
	Scorer       *User          `gorm:"foreignKey:ScorerID"`
	Rubric       *ScoringRubric `gorm:"foreignKey:RubricID"`
}

func (ManualEvaluation) TableName() string {
	return "manual_evaluations"
}
