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
}
