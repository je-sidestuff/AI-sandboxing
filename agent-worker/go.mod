module agent-worker

go 1.21

require (
	github.com/google/uuid v1.6.0
	github.com/je-sidestuff/AI-sandboxing/pkg/filestory v0.0.0
)

replace github.com/je-sidestuff/AI-sandboxing/pkg/filestory => ../pkg/filestory
