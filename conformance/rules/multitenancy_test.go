package rules_test

import (
	"reflect"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestTenantIDsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.TenantIDs()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxTenantIDLength {
		t.Fatalf("maxLength = %d, want %d", vectors.MaxLength, rules.MaxTenantIDLength)
	}
	atLimit, overLimit := false, false
	for _, id := range vectors.Valid {
		if !rules.IsTenantID(id) {
			t.Errorf("%q must be a valid tenant ID", id)
		}
		atLimit = atLimit || len(id) == vectors.MaxLength
	}
	for _, id := range vectors.Invalid {
		if rules.IsTenantID(id) {
			t.Errorf("%q must be an invalid tenant ID", id)
		}
		overLimit = overLimit || len(id) == vectors.MaxLength+1
	}
	if !atLimit || !overLimit {
		t.Errorf("the vector must keep a tenant ID of exactly %d characters (found: %v) and one of %d (found: %v)", vectors.MaxLength, atLimit, vectors.MaxLength+1, overLimit)
	}
}

func TestTenantResolutionFollowsTheVectors(t *testing.T) {
	vectors, err := conformance.TenantResolution()
	if err != nil {
		t.Fatal(err)
	}
	tenants := map[string]rules.TenantEntry{}
	for _, tenant := range vectors.Tenants {
		if !rules.IsTenantID(tenant.ID) {
			t.Errorf("stored tenant %q must have a valid ID", tenant.ID)
		}
		tenants[tenant.ID] = rules.TenantEntry{DisplayName: tenant.DisplayName, Enabled: tenant.Enabled}
	}
	for _, c := range vectors.Cases {
		got := rules.ResolveTenant(rules.TenantCall{
			Authenticated:        c.Authenticated,
			Claims:               c.Claims,
			Baggage:              c.Baggage,
			PlatformCall:         c.PlatformCall,
			DefaultTenant:        c.Options.DefaultTenant,
			RequireTenant:        c.Options.RequireTenant,
			ValidateAgainstStore: c.Options.ValidateAgainstStore,
			StoreUnavailable:     c.StoreUnavailable,
			Tenants:              tenants,
		})
		want := rules.TenantDecision{Tenant: c.Expected.Tenant, DisplayName: c.Expected.DisplayName, Error: c.Expected.Error}
		if !reflect.DeepEqual(got, want) {
			t.Errorf("%s: decision = %s, want %s", c.Name, describeTenantDecision(got), describeTenantDecision(want))
		}
		if c.Expected.Error != "" && !rules.IsErrorCode(c.Expected.Error) {
			t.Errorf("%s: %q is not an error code", c.Name, c.Expected.Error)
		}
	}
}

// describeTenantDecision 把裁定写成一行，指针按值打印。
func describeTenantDecision(decision rules.TenantDecision) string {
	text := func(value *string) string {
		if value == nil {
			return "<none>"
		}
		return *value
	}
	if decision.Error != "" {
		return "error " + decision.Error
	}
	return "tenant " + text(decision.Tenant) + " (" + text(decision.DisplayName) + ")"
}

func TestTenantShardsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.TenantShards()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.ConnectionStringProperty != rules.TenantShardConnectionStringProperty || vectors.ProviderProperty != rules.TenantShardProviderProperty || vectors.PassThroughPrefix != rules.TenantShardPassThroughPrefix {
		t.Errorf("property names = %q, %q, %q; want %q, %q, %q", vectors.ConnectionStringProperty, vectors.ProviderProperty, vectors.PassThroughPrefix,
			rules.TenantShardConnectionStringProperty, rules.TenantShardProviderProperty, rules.TenantShardPassThroughPrefix)
	}
	for _, c := range vectors.Cases {
		shard, sharded := rules.TenantShardFromProperties(c.Properties)
		if sharded != (c.Shard != nil) {
			t.Errorf("%s: sharded = %v, want %v", c.Name, sharded, c.Shard != nil)
			continue
		}
		if c.Shard == nil {
			continue
		}
		provider := ""
		if c.Shard.Provider != nil {
			provider = *c.Shard.Provider
		}
		if shard.ConnectionString != c.Shard.ConnectionString || shard.Provider != provider || !reflect.DeepEqual(shard.Properties, c.Shard.Properties) {
			t.Errorf("%s: shard = %+v, want %+v with provider %q", c.Name, shard, *c.Shard, provider)
		}
	}
}
