//go:build !windows

package corebin

import _ "embed"

//go:embed bin/core
var data []byte

const execName = "olcrtc-core"
