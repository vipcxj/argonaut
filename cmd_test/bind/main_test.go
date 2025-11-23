package bind

import (
	"flag"
	"testing"

	"github.com/google/go-cmdtest"
	"github.com/vipcxj/argonaut/cmd"
)

var update = flag.Bool("update", false, "update test files with results")

func TestCLI(t *testing.T) {
	ts, err := cmdtest.Read("testdata")
	if err != nil {
		t.Fatal(err)
	}
	ts.Commands["argonaut"] = cmdtest.InProcessProgram("argonaut", cmd.Execute)
	ts.Run(t, *update)
}
