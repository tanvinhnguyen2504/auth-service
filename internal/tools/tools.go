// Package tools defines the tool set exposed to the model. Empty for now; add
// each tool with toolrunner.NewBetaToolFromJSONSchema.
package tools

import "github.com/anthropics/anthropic-sdk-go"

// Registry returns every tool the agent may call.
func Registry() []anthropic.BetaTool {
	return nil
}
