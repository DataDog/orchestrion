module example.com/dep

go 1.23

require example.com/thing v1.0.0

// This mirrors github.com/DataDog/dd-trace-go/orchestrion/all/v2's own
// checkout-relative `replace` directives (e.g. `=> ../../contrib/...`): valid
// only when this module's own go.mod is treated as the main module, which
// must never happen when resolving this package's imports as a dependency.
replace example.com/thing v1.0.0 => ../does-not-exist
