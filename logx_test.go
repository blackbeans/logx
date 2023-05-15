package logx

import "testing"

func TestLogger(t *testing.T) {
	err := InitLogger("./logs", "log.xml")
	if nil != err {
		t.FailNow()
	}
	GetLogger("stdout").Infof("TestLogger %s", "1234")
}
