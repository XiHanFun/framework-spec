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
		if err != nil || keys.MaxBytes != 1024 || keys.Separator == "" || keys.ReservedSegment == "" || len(keys.Valid) == 0 || len(keys.Invalid) == 0 || len(keys.Reserved) == 0 ||
			len(keys.Prefixes.Valid) == 0 || len(keys.Prefixes.Invalid) == 0 {
			t.Errorf("blob keys: %v", err)
		}
		for group, cases := range map[string][]conformance.BlobKeyCase{"blob keys valid": keys.Valid, "blob keys invalid": keys.Invalid, "blob keys reserved": keys.Reserved} {
			var names []string
			for _, c := range cases {
				names = append(names, c.Name)
			}
			uniqueNames(t, group, names)
		}
		for group, cases := range map[string][]conformance.BlobKeyPrefixCase{"blob key prefixes valid": keys.Prefixes.Valid, "blob key prefixes invalid": keys.Prefixes.Invalid} {
			var names []string
			for _, c := range cases {
				names = append(names, c.Name)
			}
			uniqueNames(t, group, names)
		}
		return keys
	},
	"objectstorage/download-file-names.json": func(t *testing.T) any {
		names, err := conformance.DownloadFileNames()
		if err != nil || names.MaxBytes != 255 || len(names.Valid) == 0 || len(names.Invalid) == 0 {
			t.Errorf("download file names: %v", err)
		}
		for group, cases := range map[string][]conformance.DownloadFileNameCase{"download file names valid": names.Valid, "download file names invalid": names.Invalid} {
			var caseNames []string
			for _, c := range cases {
				caseNames = append(caseNames, c.Name)
			}
			uniqueNames(t, group, caseNames)
		}
		return names
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
	"scheduling/schedule-texts.json": func(t *testing.T) any {
		texts, err := conformance.JobScheduleTexts()
		if err != nil || len(texts.Valid) == 0 || len(texts.Invalid) == 0 {
			t.Errorf("schedule texts: %v", err)
		}
		var names []string
		kinds := map[string]bool{}
		for _, c := range texts.Valid {
			names = append(names, c.Name)
			kinds[c.Kind] = true
			if c.Text == "" || c.Canonical == "" {
				t.Errorf("schedule text case %q is incomplete: it needs the text and the text written back", c.Name)
			}
		}
		if len(kinds) != 4 {
			t.Errorf("schedule texts must cover manual, every, cron and once, got %v", kinds)
		}
		uniqueNames(t, "schedule texts", names)
		names = nil
		for _, c := range texts.Invalid {
			names = append(names, c.Name)
		}
		uniqueNames(t, "schedule text rejections", names)
		return texts
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
	"auditing/exclusions.json": func(t *testing.T) any {
		exclusions, err := conformance.AuditExclusions()
		if err != nil || len(exclusions.Entries.Valid) == 0 || len(exclusions.Entries.Invalid) == 0 || len(exclusions.Matches) == 0 {
			t.Errorf("audit exclusions: %v", err)
		}
		var names []string
		for _, c := range exclusions.Matches {
			names = append(names, c.Name)
			if c.Exclude == nil || c.Operation == "" {
				t.Errorf("audit exclusion case %q must have its list (an empty array for none) and an operation", c.Name)
			}
		}
		uniqueNames(t, "audit exclusions", names)
		return exclusions
	},
	"auditing/event-selection.json": func(t *testing.T) any {
		selection, err := conformance.AuditEventSelection()
		if err != nil || selection.Defaults.Selection == "" || len(selection.Selection) == 0 || len(selection.Attribution) == 0 {
			t.Errorf("audit event selection: %v", err)
		}
		var names []string
		outcomes := map[string]bool{}
		for _, c := range selection.Selection {
			names = append(names, c.Name)
			outcomes[c.Outcome] = true
			if c.Options.Selection == "" || c.Options.Exclude == nil {
				t.Errorf("audit selection case %q must carry its options", c.Name)
			}
		}
		for _, outcome := range []string{"success", "failure", "none", "throws", "canceled"} {
			if !outcomes[outcome] {
				t.Errorf("audit event selection must keep a case that ends with %q", outcome)
			}
		}
		uniqueNames(t, "audit selection", names)
		names = nil
		for _, c := range selection.Attribution {
			names = append(names, c.Name)
		}
		uniqueNames(t, "audit attribution", names)
		return selection
	},
	"workflow/workflow-names.json": func(t *testing.T) any {
		names, err := conformance.WorkflowNames()
		if err != nil {
			t.Errorf("workflow names: %v", err)
		}
		for group, grammar := range map[string]conformance.NameGrammarVectors{"definition names": names.DefinitionNames, "element ids": names.ElementIDs, "activity types": names.ActivityTypes} {
			if grammar.MaxLength <= 0 || len(grammar.Valid) == 0 || len(grammar.Invalid) == 0 {
				t.Errorf("workflow %s must have a limit, valid and invalid samples", group)
			}
		}
		return names
	},
	"workflow/status-order.json": func(t *testing.T) any {
		order, err := conformance.WorkflowStatusOrder()
		if err != nil || len(order.Definition) != 4 || len(order.Instance) != 6 {
			t.Errorf("workflow status order: %v", err)
		}
		return order
	},
	"workflow/keyword-matching.json": func(t *testing.T) any {
		keywords, err := conformance.WorkflowKeywords()
		if err != nil || keywords.FieldSeparator == "" || len(keywords.Normalize) == 0 || len(keywords.Matches) == 0 {
			t.Errorf("workflow keywords: %v", err)
		}
		var names []string
		for _, c := range keywords.Normalize {
			names = append(names, c.Name)
		}
		uniqueNames(t, "workflow keyword normalization", names)
		names = nil
		for _, c := range keywords.Matches {
			names = append(names, c.Name)
			if c.Keyword == "" || len(c.Fields) == 0 {
				t.Errorf("workflow keyword case %q must have a keyword and fields", c.Name)
			}
		}
		uniqueNames(t, "workflow keyword matches", names)
		return keywords
	},
	"workflow/workflow-event-topics.json": func(t *testing.T) any {
		events, err := conformance.WorkflowEventTopics()
		if err != nil || events.Source == "" || len(events.Topics) != 25 {
			t.Errorf("workflow event topics: %v", err)
		}
		return events
	},
	"workflow/expression-syntax.json": func(t *testing.T) any {
		syntax, err := conformance.WorkflowExpressionSyntax()
		if err != nil || syntax.ErrorCode == "" || len(syntax.Variables) == 0 || len(syntax.Values) == 0 || len(syntax.Errors) == 0 {
			t.Errorf("workflow expression syntax: %v", err)
		}
		return syntax
	},
	"workflow/expression-operators.json": func(t *testing.T) any {
		operators, err := conformance.WorkflowExpressionOperators()
		if err != nil || operators.ErrorCode == "" || len(operators.Variables) == 0 || len(operators.Values) == 0 || len(operators.Errors) == 0 {
			t.Errorf("workflow expression operators: %v", err)
		}
		return operators
	},
	"workflow/expression-functions.json": func(t *testing.T) any {
		functions, err := conformance.WorkflowExpressionFunctions()
		if err != nil || functions.ErrorCode == "" || functions.Now == "" || len(functions.Variables) == 0 || len(functions.Values) == 0 || len(functions.Errors) == 0 {
			t.Errorf("workflow expression functions: %v", err)
		}
		return functions
	},
	"workflow/expression-templates.json": func(t *testing.T) any {
		templates, err := conformance.WorkflowExpressionTemplates()
		if err != nil || templates.ErrorCode == "" || templates.Now == "" || len(templates.Variables) == 0 ||
			len(templates.Renders) == 0 || len(templates.RenderErrors) == 0 || len(templates.Conditions) == 0 || len(templates.ConditionErrors) == 0 {
			t.Errorf("workflow expression templates: %v", err)
		}
		return templates
	},
	"workflow/expression-limits.json": func(t *testing.T) any {
		limits, err := conformance.WorkflowExpressionLimits()
		if err != nil || limits.ErrorCode == "" || limits.MaxLength <= 0 || limits.MaxDepth <= 0 || limits.MaxStringLength <= 0 || limits.MaxEchoLength <= 0 || len(limits.Variables) == 0 ||
			len(limits.Blocks) == 0 || len(limits.Lengths) == 0 || len(limits.Nestings) == 0 || len(limits.Chains) == 0 || len(limits.Concatenations) == 0 || len(limits.Renderings) == 0 || len(limits.Echoes) == 0 {
			t.Errorf("workflow expression limits: %v", err)
		}
		var names []string
		for _, c := range limits.Lengths {
			names = append(names, c.Name)
		}
		for _, c := range limits.Nestings {
			names = append(names, c.Name)
		}
		for _, c := range limits.Chains {
			names = append(names, c.Name)
		}
		for _, c := range limits.Concatenations {
			names = append(names, c.Name)
		}
		for _, c := range limits.Renderings {
			names = append(names, c.Name)
		}
		for _, c := range limits.Echoes {
			names = append(names, c.Name)
		}
		uniqueNames(t, "workflow expression limits", names)
		return limits
	},
	"observability/host-usage.json": func(t *testing.T) any {
		usage, err := conformance.HostUsage()
		if err != nil || usage.SampleInterval.DefaultMs <= 0 || usage.SampleInterval.MaxMs <= 0 || len(usage.SampleInterval.Valid) == 0 || len(usage.SampleInterval.Invalid) == 0 ||
			len(usage.CPU) == 0 || len(usage.Process) == 0 || len(usage.Network) == 0 || len(usage.Memory) == 0 || len(usage.Disks) == 0 ||
			len(usage.Linux.CPUTimes) == 0 || len(usage.Linux.Memory) == 0 || len(usage.Linux.Uptime) == 0 || len(usage.Linux.Mounts) == 0 ||
			len(usage.Linux.Processors) == 0 || len(usage.Linux.GPUs) == 0 {
			t.Errorf("host usage: %v", err)
		}
		var names []string
		for _, c := range usage.CPU {
			names = append(names, "cpu "+c.Name)
		}
		for _, c := range usage.Process {
			names = append(names, "process "+c.Name)
			if c.Cores < 1 || c.ElapsedMs <= 0 {
				t.Errorf("host process case %q must have cores and an elapsed time", c.Name)
			}
		}
		for _, c := range usage.Network {
			names = append(names, "network "+c.Name)
		}
		for _, c := range usage.Memory {
			names = append(names, "memory "+c.Name)
		}
		for _, c := range usage.Disks {
			names = append(names, "disk "+c.Name)
		}
		for _, c := range usage.Linux.CPUTimes {
			names = append(names, "linux cpu times "+c.Name)
		}
		for _, c := range usage.Linux.Memory {
			names = append(names, "linux memory "+c.Name)
		}
		for _, c := range usage.Linux.Uptime {
			names = append(names, "linux uptime "+c.Name)
		}
		for _, c := range usage.Linux.Mounts {
			names = append(names, "linux mounts "+c.Name)
			if c.Disks == nil {
				t.Errorf("linux mounts case %q must list its disks (an empty array for none)", c.Name)
			}
		}
		for _, c := range usage.Linux.Processors {
			names = append(names, "linux processor "+c.Name)
			if len(c.Files) == 0 {
				t.Errorf("linux processor case %q must carry its files", c.Name)
			}
		}
		for _, c := range usage.Linux.GPUs {
			names = append(names, "linux gpu "+c.Name)
			if len(c.Files) == 0 || c.GPUs == nil {
				t.Errorf("linux gpu case %q must carry its files and list its GPUs (an empty array for none)", c.Name)
			}
		}
		uniqueNames(t, "host usage", names)
		return usage
	},
	"ai/ai-names.json": func(t *testing.T) any {
		names, err := conformance.AINames()
		if err != nil || names.MaxLength != 64 || len(names.Valid) == 0 || len(names.Invalid) == 0 {
			t.Errorf("ai names: %v", err)
		}
		return names
	},
	"ai/tool-names.json": func(t *testing.T) any {
		tools, err := conformance.AIToolNames()
		if err != nil || tools.MaxLength != 64 || len(tools.Valid) == 0 || len(tools.Invalid) == 0 || len(tools.Cases) == 0 {
			t.Errorf("ai tool names: %v", err)
		}
		var names []string
		for _, c := range tools.Cases {
			names = append(names, c.Name)
			if c.Operation == "" || c.Tool == "" || c.DescriptionKey == "" {
				t.Errorf("ai tool name case %q is incomplete", c.Name)
			}
		}
		uniqueNames(t, "ai tool names", names)
		return tools
	},
	"ai/knowledge-chunks.json": func(t *testing.T) any {
		knowledge, err := conformance.KnowledgeChunks()
		if err != nil || knowledge.Defaults.Collection == "" || knowledge.Defaults.ChunkSize < 1 || knowledge.MaxTop != 1000 || knowledge.MaxDocumentIDs != 1000 || knowledge.Code == "" ||
			len(knowledge.Chunks) == 0 || len(knowledge.RecordIDs) == 0 || len(knowledge.DefaultCollections) == 0 {
			t.Errorf("knowledge chunks: %v", err)
		}
		var names []string
		for _, c := range knowledge.Chunks {
			names = append(names, "chunks "+c.Name)
			if c.MaxChunkSize < 1 || c.Overlap < 0 || c.Chunks == nil {
				t.Errorf("knowledge chunk case %q must have a window, an overlap and its chunks (an empty array for none)", c.Name)
			}
		}
		for _, c := range knowledge.DefaultCollections {
			names = append(names, "collections "+c.Name)
		}
		uniqueNames(t, "knowledge chunks", names)
		return knowledge
	},
	"ai/provider-defaults.json": func(t *testing.T) any {
		defaults, err := conformance.AIProviderDefaults()
		if err != nil || defaults.Code == "" || defaults.ChatMessageKey == "" || defaults.EmbeddingMessageKey == "" || len(defaults.Cases) == 0 {
			t.Errorf("ai provider defaults: %v", err)
		}
		var names []string
		for _, c := range defaults.Cases {
			names = append(names, c.Name)
			if c.Providers == nil {
				t.Errorf("ai provider default case %q must list its providers (an empty array for none)", c.Name)
			}
		}
		uniqueNames(t, "ai provider defaults", names)
		return defaults
	},
	"ai/prompt-templates.json": func(t *testing.T) any {
		prompts, err := conformance.AIPromptTemplates()
		if err != nil || prompts.Code == "" || prompts.MessageKey == "" || len(prompts.Cases) == 0 || len(prompts.Missing) == 0 {
			t.Errorf("ai prompt templates: %v", err)
		}
		var names []string
		for _, c := range prompts.Cases {
			names = append(names, c.Name)
			if c.Template == "" || c.Arguments == nil {
				t.Errorf("ai prompt case %q must have a template and its arguments (an empty object for none)", c.Name)
			}
		}
		for _, c := range prompts.Missing {
			names = append(names, c.Name)
			if c.Template == "" || c.Arguments == nil {
				t.Errorf("ai prompt case %q must have a template and its arguments (an empty object for none)", c.Name)
			}
		}
		uniqueNames(t, "ai prompt templates", names)
		return prompts
	},
}
