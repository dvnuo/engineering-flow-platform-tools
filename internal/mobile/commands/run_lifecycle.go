package commands

import (
	"errors"
	"time"

	"engineering-flow-platform-tools/internal/mobile"
)

func markRunLostIfSessionGone(svc *services, st *mobile.RunState, err error) {
	if !isSessionLostError(err) {
		return
	}
	markRunLost(st)
	_ = svc.Store.SaveRun(*st)
}

func markRunLost(st *mobile.RunState) {
	now := time.Now().UTC()
	st.Status = mobile.StatusLost
	st.ControlOwner = "agent"
	st.FinishedAt = &now
	st.LatestObservationID = ""
}

func isSessionLostError(err error) bool {
	var me *mobile.Error
	return errors.As(err, &me) && me.Code == "session_lost"
}

func isRemoteSessionGone(err error) bool {
	var me *mobile.Error
	return errors.As(err, &me) && (me.Code == "session_lost" || me.Code == "not_found")
}
