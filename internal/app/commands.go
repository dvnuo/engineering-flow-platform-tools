package app

import "engineering-flow-platform-tools/internal/catalog"

func JiraCommandList() []string {
	return catalog.CommandList("jira")
}

func ConfluenceCommandList() []string {
	return catalog.CommandList("confluence")
}

func JenkinsCommandList() []string {
	return catalog.CommandList("jenkins")
}
