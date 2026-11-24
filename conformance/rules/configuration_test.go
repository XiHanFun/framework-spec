package rules_test

import (
	"reflect"
	"testing"

	"github.com/xihanfun/framework-spec/conformance"
	"github.com/xihanfun/framework-spec/conformance/rules"
)

func TestSecretNamesFollowTheVectors(t *testing.T) {
	vectors, err := conformance.SecretNames()
	if err != nil {
		t.Fatal(err)
	}
	if vectors.MaxLength != rules.MaxSecretNameLength {
		t.Fatalf("maxLength = %d, want %d", vectors.MaxLength, rules.MaxSecretNameLength)
	}
	replayGrammar(t, "secret name", vectors.MaxLength, vectors.Valid, vectors.Invalid, rules.IsSecretName)
}

func TestRemoteConfigurationDocumentsFollowTheVectors(t *testing.T) {
	vectors, err := conformance.RemoteConfigurationDocuments()
	if err != nil {
		t.Fatal(err)
	}
	if !rules.IsErrorCode(vectors.Code) {
		t.Errorf("%q is not an error code", vectors.Code)
	}
	flatten := func(format, content string) (map[string]*string, bool, bool) {
		switch format {
		case "Json":
			values, ok := rules.FlattenJSONConfiguration(content)
			return values, ok, true
		case "Properties":
			values, ok := rules.FlattenPropertiesConfiguration(content)
			return values, ok, true
		default:
			return nil, false, false
		}
	}
	for _, c := range vectors.Cases {
		values, ok, known := flatten(c.Format, c.Content)
		if !known {
			t.Errorf("%s: format %q must be Json or Properties", c.Name, c.Format)
			continue
		}
		if !ok || !reflect.DeepEqual(values, c.Values) {
			t.Errorf("%s: values = %s (accepted %v), want %s", c.Name, describeConfiguration(values), ok, describeConfiguration(c.Values))
		}
		if got := rules.ConfigurationContentVersion(c.Content); got != c.Version {
			t.Errorf("%s: version = %s, want %s", c.Name, got, c.Version)
		}
	}
	for _, c := range vectors.Invalid {
		if _, ok, known := flatten(c.Format, c.Content); !known || ok {
			t.Errorf("%s: the %s document must be rejected", c.Name, c.Format)
		}
	}
}

// describeConfiguration 把展平后的键值写成一行，空值写成 <null>。
func describeConfiguration(values map[string]*string) map[string]string {
	described := map[string]string{}
	for key, value := range values {
		if value == nil {
			described[key] = "<null>"
		} else {
			described[key] = *value
		}
	}
	return described
}
