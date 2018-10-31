package session

import (
	"fmt"

	log "github.com/sirupsen/logrus"
)

type cmdlist struct {
	ses    *Session
	tokens []string
	pe     processEntry
}

func newCmdList(ses *Session, tokens []string, pe processEntry) *cmdlist {
	return &cmdlist{
		ses:    ses,
		tokens: tokens,
		pe:     pe,
	}
}

func (cmd *cmdlist) requireAuth() *cmdlist {
	if cmd.pe == nil {
		return cmd
	}

	log.WithFields(log.Fields{"cmd": cmd}).Debug("session::cmdList::requireAuth called")

	if !cmd.ses.id.Authenticated() {
		cmd.ses.sendStatement("530 Please login with USER and PASS.")
		cmd.pe = nil
		return cmd
	}

	return cmd
}

func (cmd *cmdlist) requirePASV() *cmdlist {
	if cmd.pe == nil {
		return cmd
	}

	log.WithFields(log.Fields{"cmd": cmd}).Debug("session::cmdList::requirePASV called")

	if cmd.ses.lastDataChanneler == nil || cmd.ses.lastDataChanneler.IsClosed() {
		cmd.ses.sendStatement("425 Use PASV or EPSV first")
		cmd.pe = nil
		return cmd
	}

	return cmd
}

func (cmd *cmdlist) resetREST() *cmdlist {
	cmd.ses.lastREST = 0

	return cmd
}

func (cmd *cmdlist) resetUSER() *cmdlist {
	if !cmd.ses.id.Authenticated() {
		cmd.ses.id.SetUsername("")
	}
	return cmd
}

func (cmd *cmdlist) Execute() bool {
	log.WithFields(log.Fields{"cmd": cmd}).Debug("session::cmdList::Execute called")
	if cmd.pe == nil {
		return false
	}

	return cmd.pe(cmd.tokens)
}

func (cmd *cmdlist) String() string {
	return fmt.Sprintf("{ses:%v, tokens:%v, pe:%v}", cmd.ses, cmd.tokens, cmd.pe)
}
