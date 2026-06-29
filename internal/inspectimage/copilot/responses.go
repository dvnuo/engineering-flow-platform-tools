package copilot

import (
	"context"

	"engineering-flow-platform-tools/internal/inspectimage/vision"
)

func (c *Client) Responses(ctx context.Context, req vision.Request) (map[string]any, error) {
	var raw map[string]any
	if err := c.postJSON(ctx, "/responses", req, &raw); err != nil {
		return nil, err
	}
	return raw, nil
}
