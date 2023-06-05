package Logger

import (
	"testing"
)

func TestLog_SetLogger(t *testing.T) {
	LogClient := NewLogger()
	LogClient.SetLogger(Info, "../LogFile", 6)
	LogClient.GetConf()
	LogClient.Infof("test error : %s", "test")
}
