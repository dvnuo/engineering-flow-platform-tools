package commands

import (
	"errors"
	"strings"
	"time"

	"engineering-flow-platform-tools/internal/mobile"
	"engineering-flow-platform-tools/internal/output"
)

func markRunLostIfSessionGone(svc *services, st *mobile.RunState, err error) {
	if !isSessionLostError(err) {
		return
	}
	markRunLost(st, err)
	_ = svc.Store.SaveRun(*st)
}

func markRunLost(st *mobile.RunState, causes ...error) {
	markRunTerminal(st, mobile.StatusLost, "remote session is no longer available", firstError(causes...))
}

func markRunTerminal(st *mobile.RunState, status mobile.RunStatus, reason string, err error) {
	now := time.Now().UTC()
	st.Status = status
	st.ControlOwner = "agent"
	st.FinishedAt = &now
	st.LatestObservationID = ""
	st.StatusReason = strings.TrimSpace(reason)
	st.ProgressMessage = strings.TrimSpace(reason)
	if err != nil {
		code, message := errorCodeAndMessage(err)
		st.LastErrorCode = code
		st.LastErrorMessage = message
	}
}

func isSessionLostError(err error) bool {
	var me *mobile.Error
	return errors.As(err, &me) && me.Code == "session_lost"
}

func isRemoteSessionGone(err error) bool {
	var me *mobile.Error
	return errors.As(err, &me) && (me.Code == "session_lost" || me.Code == "not_found")
}

func errorCodeAndMessage(err error) (string, string) {
	if err == nil {
		return "", ""
	}
	var me *mobile.Error
	if errors.As(err, &me) {
		return me.Code, output.RedactString(me.Message)
	}
	return "error", output.RedactString(err.Error())
}

func firstError(errs ...error) error {
	for _, err := range errs {
		if err != nil {
			return err
		}
	}
	return nil
}
