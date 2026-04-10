package main

import (
	"encoding/json"
	"fmt"
	"os"
)

type JSONSchema struct {
	Schema      string                `json:"$schema"`
	ID          string                `json:"$id"`
	Title       string                `json:"title"`
	Description string                `json:"description"`
	Type        string                `json:"type"`
	Required    []string              `json:"required"`
	Properties  map[string]Property   `json:"properties"`
	Defs        map[string]JSONSchema `json:"$defs,omitempty"`
}

type Property struct {
	Type                 string              `json:"type,omitempty"`
	Description          string              `json:"description,omitempty"`
	Enum                 []string            `json:"enum,omitempty"`
	Default              any                 `json:"default,omitempty"`
	Required             []string            `json:"required,omitempty"`
	Properties           map[string]Property `json:"properties,omitempty"`
	AdditionalProperties *Property           `json:"additionalProperties,omitempty"`
	Items                *Property           `json:"items,omitempty"`
	Ref                  string              `json:"$ref,omitempty"`
}

func main() {
	schema := JSONSchema{
		Schema:      "https://json-schema.org/draft/2020-12/schema",
		ID:          "https://github.com/opentreehq/mit/mit.schema.json",
		Title:       "mit.yaml",
		Description: "Configuration file for mit - a multi-repo integration tool",
		Type:        "object",
		Required:    []string{"version", "workspace"},
		Properties: map[string]Property{
			"version": {
				Type:        "string",
				Description: "Config schema version",
				Enum:        []string{"1"},
				Default:     "1",
			},
			"workspace": {
				Type:        "object",
				Description: "Workspace-level metadata",
				Required:    []string{"name"},
				Properties: map[string]Property{
					"name": {
						Type:        "string",
						Description: "Name of the workspace",
					},
					"description": {
						Type:        "string",
						Description: "Optional description of the workspace",
					},
					"forge": {
						Type:        "string",
						Description: "Default forge type for all repos (can be overridden per-repo)",
						Enum:        []string{"github", "gitlab"},
					},
				},
			},
			"index": {
				Type:        "object",
				Description: "Configuration for the semantic index",
				Properties: map[string]Property{
					"ignore": {
						Type:        "array",
						Description: "Additional directory names to skip during indexing (node_modules, vendor, dist, etc. are always skipped)",
						Items:       &Property{Type: "string"},
					},
					"model": {
						Type:        "object",
						Description: "Custom embedding model (defaults to Qwen3-Embedding-0.6B-Q8_0.gguf)",
						Properties: map[string]Property{
							"url": {
								Type:        "string",
								Description: "Download URL for the GGUF model file. Filename is derived from the URL.",
							},
						},
					},
				},
			},
			"repos": {
				Type:        "object",
				Description: "Map of repository names to their configuration. The key is used as the repo name and default clone directory.",
				AdditionalProperties: &Property{
					Type:     "object",
					Required: []string{"url"},
					Properties: map[string]Property{
						"url": {
							Type:        "string",
							Description: "Git or Sapling clone URL",
						},
						"path": {
							Type:        "string",
							Description: "Local directory to clone into (defaults to the repo key name)",
						},
						"branch": {
							Type:        "string",
							Description: "Default branch to clone/track (defaults to 'main')",
							Default:     "main",
						},
						"forge": {
							Type:        "string",
							Description: "Forge type for this repo (overrides workspace-level forge)",
							Enum:        []string{"github", "gitlab"},
						},
					},
				},
			},
		},
	}

	data, err := json.MarshalIndent(schema, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	outPath := "mit.schema.json"
	if err := os.WriteFile(outPath, append(data, '\n'), 0644); err != nil {
		fmt.Fprintf(os.Stderr, "error writing %s: %v\n", outPath, err)
		os.Exit(1)
	}

	fmt.Printf("Generated %s\n", outPath)
}
