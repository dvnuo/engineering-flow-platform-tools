package metadata

const (
	ProductName = "visual"

	RendererGraph    = "offline.graph.v1"
	RendererTimeline = "offline.timeline.v1"
	RendererEvidence = "offline.evidence.v1"
	RendererMatrix   = "offline.matrix.v1"
)

var SupportedRenderers = map[string]bool{
	RendererGraph:    true,
	RendererTimeline: true,
	RendererEvidence: true,
	RendererMatrix:   true,
}

type Error struct {
	CodeValue    string
	MessageValue string
	HintValue    string
	StatusValue  int
}

func NewError(code, message, hint string, status int) *Error {
	if status == 0 {
		status = 400
	}
	return &Error{CodeValue: code, MessageValue: message, HintValue: hint, StatusValue: status}
}

func (e *Error) Error() string {
	if e == nil {
		return ""
	}
	return e.MessageValue
}

func (e *Error) Code() string {
	if e == nil {
		return ""
	}
	return e.CodeValue
}

func (e *Error) Message() string {
	if e == nil {
		return ""
	}
	return e.MessageValue
}

func (e *Error) Hint() string {
	if e == nil {
		return ""
	}
	return e.HintValue
}

func (e *Error) Status() int {
	if e == nil || e.StatusValue == 0 {
		return 400
	}
	return e.StatusValue
}
