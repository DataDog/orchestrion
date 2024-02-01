package at

import (
	"fmt"

	"github.com/datadog/orchestrion/internal/injector/singleton"
	"gopkg.in/yaml.v3"
)

type unmarshalerFn func(*yaml.Node) (InjectionPoint, error)

var unmarshalers = make(map[string]unmarshalerFn)

func Unmarshal(node *yaml.Node) (InjectionPoint, error) {
	key, value, err := singleton.Unmarshal(node)
	if err != nil {
		return nil, err
	}

	unmarshaller, found := unmarshalers[key]
	if !found {
		return nil, fmt.Errorf("line %d: unknown injection point type %q", node.Line, key)
	}

	ip, err := unmarshaller(value)
	return ip, err
}
