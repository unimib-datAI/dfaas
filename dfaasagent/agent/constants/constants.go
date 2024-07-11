package constants

// This package contains only some constants

const (
	// HAProxyMaxWeight is the maximum possible weight value that should be used
	// in the HAProxy configuration file
	HAProxyMaxWeight = 100

	// Names of the different strategies supported by the DFaaS agent
	RecalcStrategy = "recalcstrategy"
	NodeMarginStrategy = "nodemarginstrategy"
)
