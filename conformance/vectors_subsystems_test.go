package conformance_test

import (
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
)

// init 把各子系统档向量的非空核对并进 nonEmptyChecks：键是文件名，与向量文件表一一对应。
func init() {
	for name, check := range subsystemChecks {
		nonEmptyChecks[name] = check
	}
}

// uniqueNames 核对一组用例的名字非空且不重复。
func uniqueNames(t *testing.T, group string, names []string) {
	t.Helper()
	seen := map[string]bool{}
	for _, name := range names {
		if name == "" || seen[name] {
			t.Errorf("%s: case %q must have a unique name", group, name)
		}
		seen[name] = true
	}
}

// subsystemChecks 是各子系统档向量文件的非空核对。
var subsystemChecks = map[string]func(t *testing.T) any{
	"messaging/topics.json": func(t *testing.T) any {
		topics, err := conformance.Topics()
		if err != nil || topics.MaxLength != 128 || len(topics.Valid) == 0 || len(topics.Invalid) == 0 || topics.DeadLetter.DefaultSuffix == "" || len(topics.DeadLetter.Cases) == 0 {
			t.Errorf("topics: %v", err)
		}
		return topics
	},
	"messaging/envelope-extensions.json": func(t *testing.T) any {
		extensions, err := conformance.EnvelopeExtensions()
		if err != nil || extensions.Names.MaxLength != 20 || len(extensions.Names.Valid) == 0 || len(extensions.Names.Invalid) == 0 || len(extensions.Baggage) == 0 || extensions.Trace == "" || extensions.PartitionKey == "" || len(extensions.NotCarried) == 0 {
			t.Errorf("envelope extensions: %v", err)
		}
		return extensions
	},
	"messaging/instance-ids.json": func(t *testing.T) any {
		ids, err := conformance.MessagingInstanceIDs()
		if err != nil || ids.MaxLength != 64 || len(ids.Valid) == 0 || len(ids.Invalid) == 0 {
			t.Errorf("messaging instance ids: %v", err)
		}
		return ids
	},
	"messaging/outbox-backoff.json": func(t *testing.T) any {
		backoff, err := conformance.OutboxBackoff()
		if err != nil || backoff.Defaults.RetryDelayMs <= 0 || backoff.Defaults.MaxRetryDelayMs <= 0 || backoff.Defaults.MaxAttempts <= 0 || len(backoff.Cases) == 0 || len(backoff.DeadLetter) == 0 {
			t.Errorf("outbox backoff: %v", err)
		}
		var names []string
		for _, c := range backoff.Cases {
			names = append(names, c.Name)
			if c.RetryDelayMs <= 0 || c.MaxRetryDelayMs <= 0 || c.Attempts < 1 || c.DelayMs <= 0 {
				t.Errorf("outbox backoff case %q is incomplete", c.Name)
			}
		}
		uniqueNames(t, "outbox backoff", names)
		return backoff
	},
	"messaging/cloudevents-envelopes.json": func(t *testing.T) any {
		envelopes, err := conformance.CloudEventEnvelopes()
		if err != nil || envelopes.ContentType == "" || envelopes.Code == "" || len(envelopes.Valid) == 0 || len(envelopes.Invalid) == 0 {
			t.Errorf("cloudevents envelopes: %v", err)
		}
		var names []string
		for _, c := range envelopes.Valid {
			names = append(names, c.Name)
			if c.Text == "" || c.Expected.ID == "" || c.Expected.Source == "" || c.Expected.Type == "" || c.Expected.DataContentType == "" || c.Expected.Extensions == nil {
				t.Errorf("cloudevents case %q is incomplete", c.Name)
			}
			if c.Expected.Data != nil && c.Expected.DataBase64 != nil {
				t.Errorf("cloudevents case %q must not expect both inline and binary data", c.Name)
			}
		}
		for _, c := range envelopes.Invalid {
			names = append(names, c.Name)
		}
		uniqueNames(t, "cloudevents envelopes", names)
		return envelopes
	},
}
