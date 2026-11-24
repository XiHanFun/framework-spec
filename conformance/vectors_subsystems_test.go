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
	"multitenancy/tenant-ids.json": func(t *testing.T) any {
		ids, err := conformance.TenantIDs()
		if err != nil || ids.MaxLength != 64 || len(ids.Valid) == 0 || len(ids.Invalid) == 0 {
			t.Errorf("tenant ids: %v", err)
		}
		return ids
	},
	"multitenancy/tenant-resolution.json": func(t *testing.T) any {
		resolution, err := conformance.TenantResolution()
		if err != nil || len(resolution.Tenants) == 0 || len(resolution.Cases) == 0 {
			t.Errorf("tenant resolution: %v", err)
		}
		var names []string
		for _, c := range resolution.Cases {
			names = append(names, c.Name)
			if c.Claims == nil {
				t.Errorf("tenant resolution case %q must list its claims (an empty array for none)", c.Name)
			}
			if c.Expected.Error != "" && (c.Expected.Tenant != nil || c.Expected.DisplayName != nil) {
				t.Errorf("tenant resolution case %q must expect either an error or a tenant", c.Name)
			}
		}
		uniqueNames(t, "tenant resolution", names)
		return resolution
	},
	"multitenancy/tenant-shards.json": func(t *testing.T) any {
		shards, err := conformance.TenantShards()
		if err != nil || shards.ConnectionStringProperty == "" || shards.ProviderProperty == "" || shards.PassThroughPrefix == "" || len(shards.Cases) == 0 {
			t.Errorf("tenant shards: %v", err)
		}
		var names []string
		for _, c := range shards.Cases {
			names = append(names, c.Name)
			if c.Properties == nil || (c.Shard != nil && (c.Shard.ConnectionString == "" || c.Shard.Properties == nil)) {
				t.Errorf("tenant shard case %q is incomplete", c.Name)
			}
		}
		uniqueNames(t, "tenant shards", names)
		return shards
	},
	"objectstorage/container-names.json": func(t *testing.T) any {
		names, err := conformance.ContainerNames()
		if err != nil || names.MinLength != 3 || names.MaxLength != 63 || len(names.Valid) == 0 || len(names.Invalid) == 0 {
			t.Errorf("container names: %v", err)
		}
		return names
	},
	"objectstorage/blob-keys.json": func(t *testing.T) any {
		keys, err := conformance.BlobKeys()
		if err != nil || keys.MaxLength != 1024 || keys.Separator == "" || keys.ReservedSegment == "" || len(keys.Valid) == 0 || len(keys.Invalid) == 0 || len(keys.Reserved) == 0 {
			t.Errorf("blob keys: %v", err)
		}
		for group, cases := range map[string][]conformance.BlobKeyCase{"blob keys valid": keys.Valid, "blob keys invalid": keys.Invalid, "blob keys reserved": keys.Reserved} {
			var names []string
			for _, c := range cases {
				names = append(names, c.Name)
			}
			uniqueNames(t, group, names)
		}
		return keys
	},
	"objectstorage/blob-metadata.json": func(t *testing.T) any {
		metadata, err := conformance.BlobMetadata()
		if err != nil || metadata.Names.MaxLength != 64 || len(metadata.Names.Valid) == 0 || len(metadata.Names.Invalid) == 0 || metadata.MaxBytes != 2048 || len(metadata.Sets.Valid) == 0 || len(metadata.Sets.Invalid) == 0 {
			t.Errorf("blob metadata: %v", err)
		}
		var names []string
		for _, c := range append(append([]conformance.BlobMetadataCase{}, metadata.Sets.Valid...), metadata.Sets.Invalid...) {
			names = append(names, c.Name)
			if c.Metadata == nil {
				t.Errorf("blob metadata case %q must carry its metadata (an empty object for none)", c.Name)
			}
		}
		uniqueNames(t, "blob metadata", names)
		return metadata
	},
	"objectstorage/blob-content-types.json": func(t *testing.T) any {
		types, err := conformance.BlobContentTypes()
		if err != nil || types.MaxLength != 255 || types.Default == "" || len(types.Valid) == 0 || len(types.Invalid) == 0 {
			t.Errorf("blob content types: %v", err)
		}
		return types
	},
	"objectstorage/blob-ranges.json": func(t *testing.T) any {
		ranges, err := conformance.BlobRanges()
		if err != nil || ranges.Code == "" || len(ranges.Cases) == 0 {
			t.Errorf("blob ranges: %v", err)
		}
		var names []string
		satisfiable, unsatisfiable := false, false
		for _, c := range ranges.Cases {
			names = append(names, c.Name)
			satisfiable = satisfiable || c.Range != nil
			unsatisfiable = unsatisfiable || c.Range == nil
		}
		if !satisfiable || !unsatisfiable {
			t.Error("blob ranges must keep both satisfiable and unsatisfiable cases")
		}
		uniqueNames(t, "blob ranges", names)
		return ranges
	},
	"objectstorage/multipart-parts.json": func(t *testing.T) any {
		parts, err := conformance.MultipartParts()
		if err != nil || parts.MaxPartNumber != 10000 || parts.MinPartBytes != 5<<20 || parts.Code == "" || len(parts.PartNumbers.Valid) == 0 || len(parts.PartNumbers.Invalid) == 0 || len(parts.Manifests) == 0 {
			t.Errorf("multipart parts: %v", err)
		}
		var names []string
		for _, c := range parts.Manifests {
			names = append(names, c.Name)
			if c.Parts == nil {
				t.Errorf("multipart manifest %q must list its parts (an empty array for none)", c.Name)
			}
		}
		uniqueNames(t, "multipart manifests", names)
		return parts
	},
	"objectstorage/store-definitions.json": func(t *testing.T) any {
		definitions, err := conformance.BlobStoreDefinitions()
		if err != nil || definitions.Names.MaxLength != 64 || len(definitions.Names.Valid) == 0 || len(definitions.Names.Invalid) == 0 || len(definitions.Providers) == 0 || len(definitions.Same) == 0 {
			t.Errorf("store definitions: %v", err)
		}
		var names []string
		for _, c := range definitions.Same {
			names = append(names, c.Name)
			if c.Left.Settings == nil || c.Right.Settings == nil {
				t.Errorf("store definition case %q must carry the settings of both sides", c.Name)
			}
		}
		uniqueNames(t, "store definitions", names)
		return definitions
	},
	"scheduling/job-names.json": func(t *testing.T) any {
		names, err := conformance.JobNames()
		if err != nil || names.MaxLength != 128 || len(names.Valid) == 0 || len(names.Invalid) == 0 {
			t.Errorf("job names: %v", err)
		}
		return names
	},
	"scheduling/cron-expressions.json": func(t *testing.T) any {
		cron, err := conformance.CronExpressions()
		if err != nil || len(cron.Cases) == 0 || len(cron.Invalid) == 0 {
			t.Errorf("cron expressions: %v", err)
		}
		var names []string
		zoned, exhausted := false, false
		for _, c := range cron.Cases {
			names = append(names, c.Name)
			if c.Expression == "" || c.After == "" || c.Next == nil || (len(c.Next) == 0 && !c.Exhausted) {
				t.Errorf("cron case %q is incomplete: it needs an expression, a start and its occurrences (an empty array only when exhausted)", c.Name)
			}
			zoned = zoned || c.TimeZone != nil
			exhausted = exhausted || c.Exhausted
		}
		if !zoned || !exhausted {
			t.Error("cron expressions must keep cases in a time zone and cases that run out of occurrences")
		}
		uniqueNames(t, "cron cases", names)
		names = nil
		for _, c := range cron.Invalid {
			names = append(names, c.Name)
		}
		uniqueNames(t, "cron rejections", names)
		return cron
	},
	"scheduling/retry-delays.json": func(t *testing.T) any {
		delays, err := conformance.JobRetryDelays()
		if err != nil || delays.MaxRetries != 100 || delays.LimitMs <= 0 || len(delays.Waits) == 0 || len(delays.Policies) == 0 {
			t.Errorf("retry delays: %v", err)
		}
		var names []string
		for _, c := range delays.Waits {
			names = append(names, c.Name)
			if c.DelayMs <= 0 || c.Retry < 1 || c.WaitMs <= 0 {
				t.Errorf("retry wait case %q is incomplete", c.Name)
			}
		}
		uniqueNames(t, "retry waits", names)
		names = nil
		for _, c := range delays.Policies {
			names = append(names, c.Name)
		}
		uniqueNames(t, "retry policies", names)
		return delays
	},
	"scheduling/event-topics.json": func(t *testing.T) any {
		events, err := conformance.SchedulingEventTopics()
		if err != nil || events.Source == "" || len(events.Topics) != 9 {
			t.Errorf("scheduling event topics: %v", err)
		}
		return events
	},
	"localization/locale-names.json": func(t *testing.T) any {
		names, err := conformance.LocaleNames()
		if err != nil || names.MaxLength != 35 || names.Default == "" || len(names.BuiltIn) != 10 || len(names.Valid) == 0 || names.Longest == "" || len(names.Invalid) == 0 {
			t.Errorf("locale names: %v", err)
		}
		return names
	},
	"localization/locale-negotiation.json": func(t *testing.T) any {
		negotiation, err := conformance.LocaleNegotiation()
		if err != nil || len(negotiation.Negotiation) == 0 || len(negotiation.Fallback) == 0 {
			t.Errorf("locale negotiation: %v", err)
		}
		var names []string
		for _, c := range negotiation.Negotiation {
			names = append(names, c.Name)
			if len(c.Enabled) == 0 || c.Default == "" || c.Candidates == nil || c.Locale == "" {
				t.Errorf("negotiation case %q is incomplete", c.Name)
			}
		}
		uniqueNames(t, "locale negotiation", names)
		names = nil
		for _, c := range negotiation.Fallback {
			names = append(names, c.Name)
			if len(c.Enabled) == 0 || c.Default == "" || c.Locale == "" || len(c.Chain) == 0 {
				t.Errorf("fallback case %q is incomplete", c.Name)
			}
		}
		uniqueNames(t, "locale fallback", names)
		return negotiation
	},
	"localization/catalog-resolution.json": func(t *testing.T) any {
		resolution, err := conformance.CatalogResolution()
		if err != nil || resolution.Code == "" || len(resolution.Cases) == 0 {
			t.Errorf("catalog resolution: %v", err)
		}
		var names []string
		conflicting, missing := false, false
		for _, c := range resolution.Cases {
			names = append(names, c.Name)
			if c.Default == "" || len(c.Catalogs) == 0 || c.Requests == nil || (c.Conflict && len(c.Requests) != 0) || (!c.Conflict && len(c.Requests) == 0) {
				t.Errorf("catalog resolution case %q must have catalogs and either a conflict or lookups", c.Name)
			}
			conflicting = conflicting || c.Conflict
			for _, request := range c.Requests {
				missing = missing || request.Expected == nil
			}
		}
		if !conflicting || !missing {
			t.Error("catalog resolution must keep a conflicting case and a lookup that finds nothing")
		}
		uniqueNames(t, "catalog resolution", names)
		return resolution
	},
	"configuration/secret-names.json": func(t *testing.T) any {
		names, err := conformance.SecretNames()
		if err != nil || names.MaxLength != 256 || len(names.Valid) == 0 || len(names.Invalid) == 0 {
			t.Errorf("secret names: %v", err)
		}
		return names
	},
	"configuration/remote-documents.json": func(t *testing.T) any {
		documents, err := conformance.RemoteConfigurationDocuments()
		if err != nil || documents.Code == "" || len(documents.Cases) == 0 || len(documents.Invalid) == 0 {
			t.Errorf("remote documents: %v", err)
		}
		var names []string
		formats := map[string]bool{}
		for _, c := range documents.Cases {
			names = append(names, c.Name)
			formats[c.Format] = true
			if c.Version == "" || c.Values == nil {
				t.Errorf("remote document case %q must carry its version and values (an empty object for none)", c.Name)
			}
		}
		for _, c := range documents.Invalid {
			names = append(names, c.Name)
		}
		if !formats["Json"] || !formats["Properties"] {
			t.Error("remote documents must keep cases in both formats")
		}
		uniqueNames(t, "remote documents", names)
		return documents
	},
	"persistence/index-names.json": func(t *testing.T) any {
		names, err := conformance.IndexNames()
		if err != nil || names.MaxLength != 63 || names.Prefix == "" || names.UniquePrefix == "" || len(names.Abbreviations) == 0 || len(names.Names) == 0 || len(names.ExplicitNames.Valid) == 0 || len(names.ExplicitNames.Invalid) == 0 {
			t.Errorf("index names: %v", err)
		}
		var cases []string
		for _, c := range names.Names {
			cases = append(cases, c.Name)
			if c.Table == "" || len(c.Properties) == 0 {
				t.Errorf("index name case %q must have a table and properties", c.Name)
			}
		}
		uniqueNames(t, "index names", cases)
		return names
	},
	"persistence/snowflake-ids.json": func(t *testing.T) any {
		ids, err := conformance.SnowflakeIDs()
		if err != nil || ids.DefaultEpoch == "" || ids.TimestampBits != 41 || ids.WorkerBits != 10 || ids.SequenceBits != 12 || ids.MaxWorker != 1023 || ids.MaxSequence != 4095 || ids.WorkerLockPrefix == "" || len(ids.Cases) == 0 || len(ids.OutOfRange) == 0 {
			t.Errorf("snowflake ids: %v", err)
		}
		var cases []string
		for _, c := range ids.Cases {
			cases = append(cases, c.Name)
			if c.Epoch == "" || c.Value == "" || c.Milliseconds == "" || c.Time == "" {
				t.Errorf("snowflake case %q is incomplete", c.Name)
			}
		}
		for _, c := range ids.OutOfRange {
			cases = append(cases, c.Name)
		}
		uniqueNames(t, "snowflake ids", cases)
		return ids
	},
}
