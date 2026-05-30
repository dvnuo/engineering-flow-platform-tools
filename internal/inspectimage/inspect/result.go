package inspect

import "engineering-flow-platform-tools/internal/inspectimage/imagecheck"

type Result struct {
	Tool      string               `json:"tool"`
	Provider  string               `json:"provider"`
	Model     string               `json:"model"`
	Reasoning string               `json:"reasoning"`
	Image     imagecheck.ImageInfo `json:"image"`
	Result    any                  `json:"result"`
	Warnings  []string             `json:"warnings"`
}

type Options struct {
	ImagePath     string
	Prompt        string
	PromptFile    string
	Model         string
	Reasoning     string
	Preset        string
	TimeoutSecond int
	ConfigPath    string
}
