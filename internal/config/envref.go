package config

import "engineering-flow-platform-tools/internal/configenv"

// MissingEnvReferenceError remains available from the shared config package
// for callers that need to classify config_env_missing failures.
type MissingEnvReferenceError = configenv.MissingEnvReferenceError
