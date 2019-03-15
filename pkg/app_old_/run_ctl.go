package app_old

import (
	"encoding/json"

	"git.parallelcoin.io/pod/cmd/ctl"
	cl "git.parallelcoin.io/pod/pkg/util/cl"
)

func runCtl(
	args []string,
	cc *ctl.Config,
) {

	j, _ := json.MarshalIndent(cc, "", "  ")
	log <- cl.Tracef{"running with configuration:\n%s", string(j)}
	ctl.Main(args, cc)
}