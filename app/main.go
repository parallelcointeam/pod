package app

import (
	"github.com/tucnak/climax"
)

// PodApp is the climax main app controller for pod
var PodApp = climax.Application{
	Name:     "pod",
	Brief:    "multi-application launcher for Parallelcoin Pod",
	Version:  Version(),
	Commands: []climax.Command{},
	Topics:   []climax.Topic{},
	Groups:   []climax.Group{},
	Default:  GUICommand.Handle,
}

// Main is the real pod main
func Main() int {
	PodApp.AddCommand(CtlCommand)
	PodApp.AddCommand(NodeCommand)
	PodApp.AddCommand(WalletCommand)
	PodApp.AddCommand(ShellCommand)
	PodApp.AddCommand(ConfCommand)
	PodApp.AddCommand(VersionCommand)
	PodApp.AddCommand(GUICommand)
	return PodApp.Run()
}
