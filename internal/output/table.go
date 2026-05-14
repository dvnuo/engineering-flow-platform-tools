package output

import "fmt"

func RenderTable(env Envelope) string {
	if env.OK {
		return fmt.Sprintf("ok=true instance=%s", env.Instance)
	}
	return fmt.Sprintf("ok=false code=%s message=%s", env.Error.Code, env.Error.Message)
}
