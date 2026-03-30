package main

import (
	"github.com/mwantia/forge-plugin-skills/internal/skills"
	"github.com/mwantia/forge-sdk/pkg/plugins/grpc"
)

func main() {
	grpc.Serve(skills.NewSkillsDriver)
}
