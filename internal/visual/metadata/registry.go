package metadata

const (
	ProductName = "visual"

	RendererGraph                  = "offline.graph.v1"
	RendererTimeline               = "offline.timeline.v1"
	RendererEvidence               = "offline.evidence.v1"
	RendererMatrix                 = "offline.matrix.v1"
	RendererArchitectureIsometric  = "offline.architecture.isometric.v1"

	RendererUMLSequence  = "offline.uml.sequence.3d.v1"
	RendererUMLClass     = "offline.uml.class.2_5d.v1"
	RendererUMLState     = "offline.uml.state.3d.v1"
	RendererUMLActivity  = "offline.uml.activity.3d.v1"
	RendererUMLComponent = "offline.uml.component.3d.v1"
)

var SupportedRenderers = map[string]bool{
	RendererGraph:                 true,
	RendererTimeline:              true,
	RendererEvidence:              true,
	RendererMatrix:                true,
	RendererArchitectureIsometric: true,

	RendererUMLSequence:  true,
	RendererUMLClass:     true,
	RendererUMLState:     true,
	RendererUMLActivity:  true,
	RendererUMLComponent: true,
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
