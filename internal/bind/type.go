//go:generate go run github.com/dmarkham/enumer -type=ShellType -trimprefix=ShellType -transform=kebab
package bind

type FlagSpec struct {
	ShortName   string
	Default     []string
	Choices     []string
	Required    bool
	Multi       bool
	MultiFormat []string
	Helper      string
	EnvName     string
	Export      bool
	Value       []string
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
}

// ENUM(auto, sh, powershell, cmd)
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
