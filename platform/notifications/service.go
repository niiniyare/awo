package notifications

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"text/template"
	"time"

	"github.com/google/uuid"

	"awo.so/awo/def"
	"awo.so/awo/driver"
	"awo.so/awo/filter"
	"awo.so/awo/runtime"
)

// Service sends notifications and manages read state.
type Service struct {
	notifications driver.EntityRepository[*def.EntityRecord]
	templates     driver.EntityRepository[*def.EntityRecord]
}

// NewService creates a notification service.
func NewService(
	notifications driver.EntityRepository[*def.EntityRecord],
	templates driver.EntityRepository[*def.EntityRecord],
) *Service {
	return &Service{notifications: notifications, templates: templates}
}

// SendInput carries the fields required to create a notification.
type SendInput struct {
	TenantID    uuid.UUID
	RecipientID uuid.UUID
	Channel     string // "in_app", "email", "sms", "push"
	Title       string
	Body        string
	TemplateKey string         // optional; renders Title+Body from template
	Data        map[string]any // template variables and arbitrary context
}

// Send creates a notification record. The AfterCreate hook dispatches delivery.
func (s *Service) Send(ctx context.Context, in SendInput) (*def.EntityRecord, error) {
	if in.TemplateKey != "" {
		rendered, err := s.renderTemplate(ctx, in.TemplateKey, in.Data)
		if err != nil {
			return nil, fmt.Errorf("notifications.Send: render template %q: %w", in.TemplateKey, err)
		}
		if rendered.subject != "" {
			in.Title = rendered.subject
		}
		if rendered.body != "" {
			in.Body = rendered.body
		}
	}

	dataJSON, err := json.Marshal(in.Data)
	if err != nil {
		return nil, fmt.Errorf("notifications.Send: marshal data: %w", err)
	}

	rec, err := s.notifications.Create(ctx, driver.CreateInput{
		Data: map[string]any{
			"tenant_id":    in.TenantID.String(),
			"recipient_id": in.RecipientID.String(),
			"channel":      in.Channel,
			"title":        in.Title,
			"body":         in.Body,
			"data":         dataJSON,
		},
	})
	if err != nil {
		return nil, fmt.Errorf("notifications.Send: create: %w", err)
	}
	return rec, nil
}

// MarkRead marks an in-app notification as read.
func (s *Service) MarkRead(ctx context.Context, notificationID uuid.UUID) error {
	_, err := s.notifications.Update(ctx, notificationID, driver.UpdateInput{
		Data: map[string]any{"read_at": time.Now().UTC()},
	})
	if err != nil {
		return fmt.Errorf("notifications.MarkRead %s: %w", notificationID, err)
	}
	return nil
}

// UnreadCount returns the number of unread in-app notifications for a recipient.
func (s *Service) UnreadCount(ctx context.Context, recipientID uuid.UUID) (int64, error) {
	f := filter.And(
		filter.Eq("recipient_id", recipientID.String()),
		filter.Eq("channel", "in_app"),
		filter.IsNull("read_at"),
	)
	count, err := s.notifications.Count(ctx, f)
	if err != nil {
		return 0, fmt.Errorf("notifications.UnreadCount: %w", err)
	}
	return count, nil
}

// MarkAllRead marks all unread in-app notifications for a recipient as read.
func (s *Service) MarkAllRead(ctx context.Context, recipientID uuid.UUID) (int64, error) {
	f := filter.And(
		filter.Eq("recipient_id", recipientID.String()),
		filter.Eq("channel", "in_app"),
		filter.IsNull("read_at"),
	)
	n, err := s.notifications.BulkUpdate(ctx, f, driver.Patch{
		Set: map[string]any{"read_at": time.Now().UTC()},
	})
	if err != nil {
		return 0, fmt.Errorf("notifications.MarkAllRead: %w", err)
	}
	return n, nil
}

type renderedTemplate struct {
	subject string
	body    string
}

func (s *Service) renderTemplate(ctx context.Context, key string, data map[string]any) (renderedTemplate, error) {
	results, _, err := s.templates.Query(ctx, filter.And(
		filter.Eq("key", key),
		filter.Eq("active", true),
	), driver.WithSkipCount())
	if err != nil {
		return renderedTemplate{}, fmt.Errorf("query: %w", err)
	}
	if len(results) == 0 {
		return renderedTemplate{}, &runtime.NotFoundError{EntityName: "platform_notification_template", ID: key}
	}
	tmplRec := results[0]

	var rt renderedTemplate
	if subjectTmpl := tmplRec.GetString("subject_template"); subjectTmpl != "" {
		rendered, err := renderGoTemplate(subjectTmpl, data)
		if err != nil {
			return renderedTemplate{}, fmt.Errorf("render subject: %w", err)
		}
		rt.subject = rendered
	}
	if bodyTmpl := tmplRec.GetString("body_template"); bodyTmpl != "" {
		rendered, err := renderGoTemplate(bodyTmpl, data)
		if err != nil {
			return renderedTemplate{}, fmt.Errorf("render body: %w", err)
		}
		rt.body = rendered
	}
	return rt, nil
}

func renderGoTemplate(tmplStr string, data map[string]any) (string, error) {
	t, err := template.New("").Parse(tmplStr)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}
