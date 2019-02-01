package app

import (
	"encoding/json"
	"sync"
	"time"

	cl "git.parallelcoin.io/pod/pkg/clog"
	"git.parallelcoin.io/pod/pkg/interrupt"
)

func runShell() (out int) {
	j, _ := json.MarshalIndent(ShellConfig, "", "  ")
	log <- cl.Tracef{"running with configuration:\n%s", string(j)}
	var wg sync.WaitGroup
	go func() {
		wg.Add(1)
		out = runNode(ShellConfig.Node, ShellConfig.nodeActiveNet)
		wg.Done()
	}()
	time.Sleep(time.Second * 5)
	go func() {
		wg.Add(1)
		out = runWallet(ShellConfig.Wallet, ShellConfig.walletActiveNet)
		wg.Done()
	}()
	wg.Wait()
	<-interrupt.HandlersDone
	return 0
}
