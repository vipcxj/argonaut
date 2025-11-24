package bind

import (
	"testing"

	"github.com/vipcxj/argonaut/cmd"
	"github.com/vipcxj/argonaut/cmdtest"
)

func TestCLI(t *testing.T) {
	ts, err := cmdtest.Read("testdata")
	if err != nil {
		t.Fatal(err)
	}
	ts.Register("argonaut", cmd.Execute)
	ts.Run(t, true)
}
