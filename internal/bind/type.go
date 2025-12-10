//go:generate go run github.com/dmarkham/enumer -type=ShellType -trimprefix=ShellType -transform=kebab
//go:generate go run github.com/dmarkham/enumer -type=HelpSinkType -trimprefix=HelpSink -transform=kebab
package bind

type FlagSpec struct {
	ShortName     string
	Default       []string
	NoOptDefValue string
	Choices       []string
	Required      bool
	Multi         bool
	MultiFormat   []string
	Helper        string
	EnvName       string
	Export        bool
	Value         []string
}

type CmdSpec struct {
	Name        string
	ShortDesc   string
	LongDesc    string
	Interactive bool
	EnvPrefix   string
	Flags       map[string]*FlagSpec
	Debug       bool
	ArgsRange   IntRange
	ArgsChoices [][]string
	ArgsValue   []string
	ShellType   ShellType
	HelpVar     string
	HelpExport  bool
}

type ShellType int

const (
	ShellTypeAuto ShellType = iota
	ShellTypeSh
	ShellTypePowershell
	ShellTypeCmd
)

type ShellInfo struct {
	Type ShellType
	Name string
	Path string
}
