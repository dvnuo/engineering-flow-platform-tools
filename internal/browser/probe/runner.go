package probe

import "context"

type Runner interface {
	Probe(ctx context.Context, opts ProbeOptions) (ProbeResult, error)
}
