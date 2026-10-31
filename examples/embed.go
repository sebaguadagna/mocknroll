// Package examples embeds the response files of the example mocks, so they
// can be created in any working directory on first run.
package examples

import "embed"

//go:embed users.json orders.json
var Files embed.FS
