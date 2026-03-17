package l402

import (
	"github.com/btcsuite/btclog/v2"
	"github.com/lightninglabs/lnget/build"
)

// Subsystem defines the logging sub system name for the l402 package.
const Subsystem = "L402"

// log is the package-level logger for the l402 subsystem.
var log btclog.Logger

func init() {
	log = build.RegisterSubLogger(Subsystem, UseLogger)
}

// UseLogger replaces the package-level logger.
func UseLogger(logger btclog.Logger) {
	log = logger
}
