package wire

import "testing"

func TestOpcodeConstants(t *testing.T) {
	if OpMsg == 0 && OpReply == 0 {
		// just ensure package symbols exist — values are non-zero typically
	}
	_ = OpMsg
	_ = OpQuery
}
