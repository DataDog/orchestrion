package gcp_pubsub

import (
	"orchestrion/integration/harness"
	"testing"
)

func Test(t *testing.T) {
	tc := &TestCase{}
	harness.RunTest(t, tc)
}
