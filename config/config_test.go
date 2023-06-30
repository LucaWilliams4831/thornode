package config

import (
	"reflect"
	"strings"
	"testing"

	. "gopkg.in/check.v1"
	"gopkg.in/yaml.v2"
)

func TestPackage(t *testing.T) { TestingT(t) }

type Test struct{}

var _ = Suite(&Test{})

func (Test) TestAllDefaultDefined(c *C) {
	// In order to override configuration values, defaults must first be defined
	// in the default YAML file. This test ensures all fields have defaults defined.

	confMap := map[interface{}]interface{}{}
	err := yaml.Unmarshal(defaultConfig, &confMap)
	c.Assert(err, IsNil)

	// recursive check defaults for all fields in config struct
	check(c, []string{}, confMap, reflect.TypeOf(Config{}))
}

func check(c *C, path []string, cm map[interface{}]interface{}, t reflect.Type) {
	// ensure all config fields are defined
	for i := 0; i < t.NumField(); i++ {
		tag := t.Field(i).Tag.Get("mapstructure")
		tagPath := strings.Join(append(path, tag), ".")

		// skip some fields, since there are environment variables we won't override
		if strings.HasPrefix(tagPath, "bifrost.signer.block_scanner") {
			continue
		}
		if strings.HasPrefix(tagPath, "bifrost.thorchain.back_off") {
			continue
		}
		if t.Field(i).Name == "SignerPasswd" {
			continue
		}

		// assert the field is defined in config
		if _, ok := cm[tag]; !ok {
			c.Fatalf("missing default for %s %s", tagPath, t.Field(i).Type)
		}

		// trunk-ignore-all(golangci-lint/forcetypeassert)
		switch t.Field(i).Type.Kind() {

		case reflect.Struct: // recurse if this is a nested struct
			check(c, append(path, tag), cm[tag].(map[interface{}]interface{}), t.Field(i).Type)

		case reflect.Map: // recurse on each value if this is a map
			for k, v := range cm[tag].(map[interface{}]interface{}) {
				check(c, append(path, tag, k.(string)), v.(map[interface{}]interface{}), t.Field(i).Type.Elem())
			}
		}
	}

	// ensure all defaults are defined in config
	for k := range cm {
		found := false
		for i := 0; i < t.NumField(); i++ {
			if t.Field(i).Tag.Get("mapstructure") == k {
				found = true
				break
			}
		}
		if !found {
			c.Fatalf("unknown mapping: %s", strings.Join(append(path, k.(string)), "."))
		}
	}
}
