package ai

import _ "embed"

const PromptVersion = "m3-2026-10-13"

//go:embed prompts/analyze.txt
var analyzePrompt string

//go:embed prompts/script.txt
var scriptPrompt string
