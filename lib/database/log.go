package database

import (
	"git.parallelcoin.io/pod/lib/clog"
)

// Log is the logger for the peer package
var Log = cl.NewSubSystem("lib/database", "info")
var log = Log.Ch

// UseLogger uses a specified Logger to output package logging info.
func UseLogger(logger *cl.SubSystem) {
	Log = logger
	log = Log.Ch
}
