package executor

// Result represents the outcome of a task execution.
type Result struct {
	// RepoName is the name of the repo this result belongs to.
	RepoName string `json:"repo"`
	// Success indicates if the task succeeded.
	Success bool `json:"success"`
	// Output contains any output from the task.
	Output string `json:"output,omitempty"`
	// Error contains the error message if the task failed.
	Error string `json:"error,omitempty"`
	// Data holds arbitrary result data.
	Data any `json:"data,omitempty"`
}
