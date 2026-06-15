package store

import (
	"context"
	"time"
)

type LogicOp string

const (
	LogicAND LogicOp = "AND"
	LogicOR  LogicOp = "OR"
)

type ConditionField string

const (
	FieldFrom          ConditionField = "from"
	FieldTo            ConditionField = "to"
	FieldCC            ConditionField = "cc"
	FieldSubject       ConditionField = "subject"
	FieldBody          ConditionField = "body"
	FieldHeader        ConditionField = "header"
	FieldTag           ConditionField = "tag"
	FieldSize          ConditionField = "size"
	FieldHasAttachment ConditionField = "has_attachment"
)

type ConditionOperator string

const (
	OpContains    ConditionOperator = "contains"
	OpNotContains ConditionOperator = "not_contains"
	OpEquals      ConditionOperator = "equals"
	OpNotEquals   ConditionOperator = "not_equals"
	OpStartsWith  ConditionOperator = "starts_with"
	OpEndsWith    ConditionOperator = "ends_with"
	OpRegex       ConditionOperator = "regex"
	OpGT          ConditionOperator = "gt"
	OpLT          ConditionOperator = "lt"
	OpExists      ConditionOperator = "exists"
)

type ActionType string

const (
	ActionTag       ActionType = "tag"
	ActionRemoveTag ActionType = "remove_tag"
	ActionColor     ActionType = "color"
	ActionMarkRead  ActionType = "mark_read"
	ActionStar      ActionType = "star"
	ActionDelete    ActionType = "delete"
	ActionWebhook   ActionType = "webhook"
	ActionFolder    ActionType = "folder"
)

type Condition struct {
	Field     ConditionField    `json:"field"`
	Operator  ConditionOperator `json:"operator"`
	Value     string            `json:"value"`
	HeaderKey string            `json:"header_key,omitempty"`
}

type Action struct {
	Type  ActionType `json:"type"`
	Value string     `json:"value"`
}

type RuleStats struct {
	MatchCount  int        `json:"match_count"`
	LastMatchAt *time.Time `json:"last_match_at"`
}

type Rule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Enabled     bool        `json:"enabled"`
	Priority    int         `json:"priority"`
	Logic       LogicOp     `json:"logic"`
	Conditions  []Condition `json:"conditions"`
	Actions     []Action    `json:"actions"`
	Stats       RuleStats   `json:"stats"`
	CreatedAt   time.Time   `json:"created_at"`
	UpdatedAt   time.Time   `json:"updated_at"`
}

type Email struct {
	ID          string              `json:"id"`
	MessageID   string              `json:"message_id"`
	From        string              `json:"from"`
	To          []string            `json:"to"`
	CC          []string            `json:"cc"`
	BCC         []string            `json:"bcc"`
	Subject     string              `json:"subject"`
	Text        string              `json:"text"`
	HTML        string              `json:"html"`
	RawMessage  []byte              `json:"-"`
	Attachments []Attachment        `json:"attachments"`
	Headers     map[string][]string `json:"headers"`
	Tags        []string            `json:"tags"`
	Color       string              `json:"color,omitempty"`
	Folder      string              `json:"folder,omitempty"`
	Read        bool                `json:"read"`
	Starred     bool                `json:"starred"`
	Size        int                 `json:"size"`
	ReceivedAt  time.Time           `json:"received_at"`
	SMTPLog     string              `json:"smtp_log,omitempty"`
}

type Attachment struct {
	Filename    string `json:"filename"`
	ContentType string `json:"content_type"`
	Size        int    `json:"size"`
	Data        []byte `json:"-"`
}

type SearchFilter struct {
	Query   string
	Tag     string
	Folder  string
	Read    *bool
	Starred *bool
	From    string
	To      string
	Page    int
	Limit   int
	Sort    string
}

type Event struct {
	Type    string `json:"type"`
	Payload any    `json:"payload"`
}

type Stats struct {
	Total      int   `json:"total"`
	Unread     int   `json:"unread"`
	Starred    int   `json:"starred"`
	SizeBytes  int64 `json:"size_bytes"`
	RulesCount int   `json:"rules_count"`
}

type Store interface {
	Add(ctx context.Context, email *Email) error
	Get(ctx context.Context, id string) (*Email, error)
	List(ctx context.Context, filter SearchFilter) ([]*Email, int, error)
	Update(ctx context.Context, email *Email) error
	Delete(ctx context.Context, id string) error
	DeleteAll(ctx context.Context, ids []string) error

	Stats(ctx context.Context) (Stats, error)
	Tags(ctx context.Context) (map[string]int, error)
	Folders(ctx context.Context) (map[string]int, error)
	RenameTag(ctx context.Context, oldName, newName string) error
	DeleteTag(ctx context.Context, name string) error
	RenameFolder(ctx context.Context, oldName, newName string) error
	DeleteFolder(ctx context.Context, name string) error

	ListRules(ctx context.Context) ([]*Rule, error)
	AddRule(ctx context.Context, rule *Rule) error
	GetRule(ctx context.Context, id string) (*Rule, error)
	UpdateRule(ctx context.Context, rule *Rule) error
	DeleteRule(ctx context.Context, id string) error

	Subscribe(ctx context.Context) (<-chan Event, func())
	Publish(event Event)

	Close() error
}
