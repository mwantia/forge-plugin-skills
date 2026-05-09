package main

import (
	"github.com/mwantia/forge-plugin-skills/plugin"
	"github.com/mwantia/forge-sdk/pkg/plugins/grpc"
)

func main() {
	grpc.Serve(plugin.NewSkillsDriver)
}
