package bind

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type FlagSpec struct {
	ShortName   string
	Default     []string
	Choices     []string
	Required    bool
	Multi       bool
	MultiFormat []string
	Helper      string
}

var AllowedMultiFormats = []string{"comma", "newline", "space", "json"}

func checkInStringSlice(value string, slice []string) bool {
	for _, f := range slice {
		if value == f {
			return true
		}
	}
	return false
}

func checkMultiFormat(formats []string, flag string) error {
	for _, format := range formats {
		if !checkInStringSlice(format, AllowedMultiFormats) {
			return fmt.Errorf("invalid multi format: %s for flag %s, allowed formats are: %v", format, flag, AllowedMultiFormats)
		}
	}
	if checkInStringSlice("json", formats) && len(formats) > 1 {
		return fmt.Errorf("multi format 'json' for flag %s cannot be combined with other formats", flag)
	}
	return nil
}

func splitAndTrim(s string, seps string) []string {
	isSep := func(r rune) bool { return strings.ContainsRune(seps, r) }
	parts := strings.FieldsFunc(s, isSep) // 自动丢弃空片段
	for i := range parts {
		parts[i] = strings.TrimSpace(parts[i])
	}
	return parts
}

func parseMultiValues(formats []string, rawValues []string, flag string) ([]string, error) {
	if rawValues == nil {
		return nil, nil
	}
	if len(rawValues) == 0 {
		return []string{}, nil
	}
	if len(formats) == 0 {
		return rawValues, nil
	}
	if checkInStringSlice("json", formats) {
		if len(formats) > 1 {
			return nil, fmt.Errorf("multi format 'json' for flag %s cannot be combined with other formats", flag)
		}
		var result []string
		for _, raw := range rawValues {
			raw = strings.TrimSpace(raw)
			if raw == "" {
				continue
			}
			// 尝试解析为 JSON 数组
			var arr []string
			if err := json.Unmarshal([]byte(raw), &arr); err == nil {
				result = append(result, arr...)
				continue
			}
			// 尝试解析为单个 JSON 字符串（"value"）
			var s string
			if err := json.Unmarshal([]byte(raw), &s); err == nil {
				result = append(result, s)
				continue
			}
			return nil, fmt.Errorf("invalid json multi value: %s for flag %s", raw, flag)
		}
		return result, nil
	} else {
		sepsBuilder := strings.Builder{}
		for _, format := range formats {
			switch format {
			case "comma":
				sepsBuilder.WriteString(",")
			case "newline":
				sepsBuilder.WriteString("\r\n")
			case "space":
				sepsBuilder.WriteString(" ")
			default:
				return nil, fmt.Errorf("unsupported multi format: %s for flag %s", format, flag)
			}
		}
		seps := sepsBuilder.String()
		var result []string
		for _, raw := range rawValues {
			splitValues := splitAndTrim(raw, seps)
			result = append(result, splitValues...)
		}
		return result, nil
	}
}

type CmdSpec struct {
	Name        string
	ShortDesc   string
	LongDesc    string
	Interactive bool
	Flags       map[string]*FlagSpec
	Args        int
	ArgsChoices [][]string
}

const ShortDesc = "Define, bind and validate CLI arguments, then export them as shell environment variables"

const LongDesc = `Bind collects declarative argument specifications (defaults, allowed values, required/multi flags),
validates inputs, and supports two operation modes: auto, interactive and non-interactive.
It can prompt users with keyboard-driven selectors, enforce allowed values,
handle multi-valued flags, and finally emit shell-friendly "export" statements
so calling scripts can eval/source the output to import variables into their environment.`

// Run is the migrated command logic for the bind command.
func Run(cmd *cobra.Command, args []string) {
	var err error
	cmdArgs, userArgs := splitAtDoubleDash(args)
	spec := collectSpecs(cmd, cmdArgs, userArgs)
	realCmd := &cobra.Command{
		Use:   spec.Name,
		Short: spec.ShortDesc,
		Long:  spec.LongDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	for flagName, spec := range spec.Flags {
		if !spec.Multi {
			var defaultVar string
			if len(spec.Default) > 0 {
				defaultVar = spec.Default[0]
			} else {
				defaultVar = ""
			}
			realCmd.Flags().StringP(flagName, spec.ShortName, defaultVar, spec.Helper)
		} else {
			realCmd.Flags().StringArrayP(flagName, spec.ShortName, spec.Default, spec.Helper)
		}
		if len(spec.Choices) > 0 {
			realCmd.RegisterFlagCompletionFunc(flagName, func(cmd *cobra.Command, args []string, toComplete string) ([]cobra.Completion, cobra.ShellCompDirective) {
				var completions []cobra.Completion
				for _, choice := range spec.Choices {
					if strings.HasPrefix(choice, toComplete) {
						completions = append(completions, choice)
					}
				}
				return completions, cobra.ShellCompDirectiveDefault
			})
		}
		if spec.Required {
			realCmd.MarkFlagRequired(flagName)
		}
	}
	realCmd.SetArgs(userArgs[1:]) // skip the first arg which is the command name
	err = realCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func collectFlagsName(args []string) ([]string, error) {
	fs := pflag.NewFlagSet("flags", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.SetOutput(io.Discard)
	fs.StringSliceP("flag", "f", []string{}, "")
	fs.Parse(args)
	return fs.GetStringSlice("flag")
}

func getRepeatedFlagsName(names []string) []string {
	// 找到重复的name
	nameCount := make(map[string]int)
	for _, n := range names {
		nameCount[n]++
	}
	var repeated []string
	for n, count := range nameCount {
		if count > 1 {
			repeated = append(repeated, n)
		}
	}
	return repeated
}

func collectFlagsMulti(specs map[string]*FlagSpec, flagsName []string, argsValues []string) error {
	fs := pflag.NewFlagSet("flags", pflag.ContinueOnError)
	fs.ParseErrorsAllowlist.UnknownFlags = true
	fs.SetOutput(io.Discard)
	for _, flagName := range flagsName {
		_, exists := specs[flagName]
		if exists {
			continue
		}
		specs[flagName] = &FlagSpec{
			Default:     []string{},
			Choices:     []string{},
			MultiFormat: []string{AllowedMultiFormats[0]},
		}
		flag_name := fmt.Sprintf("flag-%s-multi", flagName)
		fs.BoolP(flag_name, "", false, "")
		flag_name = fmt.Sprintf("flag-%s-multi-format", flagName)
		fs.StringSliceP(flag_name, "", AllowedMultiFormats[0:1], "")
	}
	fs.Parse(argsValues)
	for _, flagName := range flagsName {
		flag_name := fmt.Sprintf("flag-%s-multi", flagName)
		multi, err := fs.GetBool(flag_name)
		if err != nil {
			return err
		}
		specs[flagName].Multi = multi
		flag_name = fmt.Sprintf("flag-%s-multi-format", flagName)
		multiFormat, err := fs.GetStringSlice(flag_name)
		if err != nil {
			return err
		}
		specs[flagName].MultiFormat = multiFormat
	}
	return nil
}

func collectSpecs(cmd *cobra.Command, bindArgs []string, userArgs []string) *CmdSpec {
	rootCmd := cmd.Root()
	specs := &CmdSpec{
		Flags: make(map[string]*FlagSpec),
	}
	flagsName, err := collectFlagsName(bindArgs)
	if err != nil {
		cmd.PrintErrln(fmt.Sprintf("%s %v", cmd.ErrPrefix(), err))
		os.Exit(1)
	}
	err = collectFlagsMulti(specs.Flags, flagsName, bindArgs)
	if err != nil {
		cmd.PrintErrln(fmt.Sprintf("%s %v", cmd.ErrPrefix(), err))
		os.Exit(1)
	}
	virtualRootCmd := &cobra.Command{
		Use:   rootCmd.Use,
		Short: rootCmd.Short,
		Long:  rootCmd.Long,
		Run:   func(cmd *cobra.Command, args []string) {},
	}

	example := `  [---in shell script: my-shell.sh---]
  %s bind \
    --flag flag-1 --flag-flag-1-default 1 --flag-flag-1-choices 1,2,3 \
    --flag flag-2 --flag-flag-2-required --flag-flag-2-choices a,b,c \
    -- $0 "$@"

  [---then use my-shell.sh like this:--]
  ./my-shell.sh --flag-1 2 --flag-2 b`

	bindCmd := &cobra.Command{
		Use:     "bind [flags] -- [user args include $0]",
		Args:    cobra.ExactArgs(0),
		Example: fmt.Sprintf(example, rootCmd.Name()),
		Short:   ShortDesc,
		Long:    LongDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := cmd.Flags().GetString("name")
			if err != nil {
				return err
			}
			specs.Name = name
			longDesc, err := cmd.Flags().GetString("long")
			if err != nil {
				return err
			}
			specs.LongDesc = longDesc
			shortDesc, err := cmd.Flags().GetString("short")
			if err != nil {
				return err
			}
			specs.ShortDesc = shortDesc
			allowRepeated, err := cmd.Flags().GetBool("allow-repeated-flags")
			if err != nil {
				return err
			}
			if !allowRepeated {
				repeated := getRepeatedFlagsName(flagsName)
				if len(repeated) > 0 {
					return fmt.Errorf("repeated argument names: %v", repeated)
				}
			}
			interactive, err := cmd.Flags().GetBool("interactive")
			if err != nil {
				return err
			}
			specs.Interactive = interactive
			argsCount, err := cmd.Flags().GetInt("args")
			if err != nil {
				return err
			}
			if argsCount < -1 {
				return fmt.Errorf("invalid args count: %d, should be -1 or non-negative", argsCount)
			}
			specs.Args = argsCount
			for flagName, spec := range specs.Flags {
				err = checkMultiFormat(spec.MultiFormat, flagName)
				if err != nil {
					return err
				}
				shortFlag := fmt.Sprintf("flag-%s-short", flagName)
				defaultFlag := fmt.Sprintf("flag-%s-default", flagName)
				choicesFlag := fmt.Sprintf("flag-%s-choices", flagName)
				requiredFlag := fmt.Sprintf("flag-%s-required", flagName)
				helperFlag := fmt.Sprintf("flag-%s-helper", flagName)
				shortValue, err := cmd.Flags().GetString(shortFlag)
				if err != nil {
					return err
				}
				spec.ShortName = shortValue
				helperValue, err := cmd.Flags().GetString(helperFlag)
				if err != nil {
					return err
				}
				spec.Helper = helperValue
				if spec.Multi {
					if defaultValues, err := cmd.Flags().GetStringArray(defaultFlag); err != nil {
						return err
					} else {
						if defaultValues, err := parseMultiValues(spec.MultiFormat, defaultValues, flagName); err != nil {
							return err
						} else {
							spec.Default = defaultValues
						}
					}
				} else {
					defaultValue, err := cmd.Flags().GetString(defaultFlag)
					if err != nil {
						return err
					}
					spec.Default = []string{defaultValue}
				}
				if choicesValue, err := cmd.Flags().GetStringArray(choicesFlag); err != nil {
					return err
				} else {
					if choicesValue, err := parseMultiValues([]string{"comma"}, choicesValue, flagName); err != nil {
						return err
					} else {
						spec.Choices = choicesValue
					}
				}
				requiredValue, err := cmd.Flags().GetBool(requiredFlag)
				if err != nil {
					return err
				}
				spec.Required = requiredValue

				if len(spec.Choices) > 0 && len(spec.Default) > 0 {
					for _, def := range spec.Default {
						if !checkInStringSlice(def, spec.Choices) {
							return fmt.Errorf("default value %s for flag %s is not in allowed choices %v", def, flagName, spec.Choices)
						}
					}
				}
			}
			return nil
		},
	}
	bindCmd.Flags().StringP("name", "n", "", "The name of the command")
	bindCmd.Flags().StringP("short", "s", "", "The short description of the command")
	bindCmd.Flags().StringP("long", "l", "", "The long description of the command")
	bindCmd.Flags().BoolP("interactive", "i", false, "Enable interactive mode for user prompts")
	bindCmd.Flags().BoolP("allow-repeated-flags", "r", false, "Allow repeated flag names")
	bindCmd.Flags().IntP("args", "a", 0, "The number of positional arguments, -1 for unlimited")
	bindCmd.Flags().StringSliceP("flag", "f", []string{}, "Name For flag")
	for flagName, spec := range specs.Flags {
		shortFlag := fmt.Sprintf("flag-%s-short", flagName)
		bindCmd.Flags().StringP(shortFlag, "", "", fmt.Sprintf("Short name for flag %s", flagName))
		helpFlag := fmt.Sprintf("flag-%s-helper", flagName)
		bindCmd.Flags().StringP(helpFlag, "", "", fmt.Sprintf("Helper text for flag %s", flagName))
		multiFlag := fmt.Sprintf("flag-%s-multi", flagName)
		bindCmd.Flags().BoolP(multiFlag, "", false, fmt.Sprintf("Whether flag %s is multi-valued", flagName))
		multiFormatFlag := fmt.Sprintf("flag-%s-multi-format", flagName)
		bindCmd.Flags().StringP(
			multiFormatFlag, "", AllowedMultiFormats[0],
			fmt.Sprintf("Multi value format for flag %s, allowed value are combined of %v or %v", flagName, strings.Join(AllowedMultiFormats[0:3], ", "), AllowedMultiFormats[3]),
		)
		defaultFlag := fmt.Sprintf("flag-%s-default", flagName)
		if spec.Multi {
			bindCmd.Flags().StringArrayP(defaultFlag, "", []string{}, fmt.Sprintf("Default values for flag %s", flagName))
		} else {
			bindCmd.Flags().StringP(defaultFlag, "", "", fmt.Sprintf("Default value for flag %s", flagName))
		}
		choicesFlag := fmt.Sprintf("flag-%s-choices", flagName)
		bindCmd.Flags().StringArrayP(choicesFlag, "", []string{}, fmt.Sprintf("Allowed choices for flag %s", flagName))
		requiredFlag := fmt.Sprintf("flag-%s-required", flagName)
		bindCmd.Flags().BoolP(requiredFlag, "", false, fmt.Sprintf("Whether flag %s is required", flagName))
	}
	virtualRootCmd.AddCommand(bindCmd)
	argsWithBind := append([]string{"bind"}, bindArgs...)
	virtualRootCmd.SetArgs(argsWithBind)

	err = virtualRootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
	// 如果只请求帮助信息，则退出成功
	if bindCmd.Flags().Changed("help") {
		os.Exit(0)
	}

	if len(userArgs) == 0 {
		bindCmd.PrintErrln(fmt.Sprintf("%s no user arguments provided after '--', at least $0 should be provided", bindCmd.ErrPrefix()))
		bindCmd.Usage()
		os.Exit(1)
	}
	if specs.Name == "" {
		specs.Name = userArgs[0]
	}
	return specs
}

// splitAtDoubleDash 在 args 中查找第一个 "--" 并返回两段切片：
// - before: "--" 之前的部分
// - after:  "--" 之后的部分（如果不存在 "--"，则返回空切片）
func splitAtDoubleDash(args []string) (before []string, after []string) {
	for i, a := range args {
		if a == "--" {
			// 复制切片以避免后续修改影响原始切片
			before = append([]string{}, args[:i]...)
			after = append([]string{}, args[i+1:]...)
			return
		}
	}
	// 未找到 "--"
	before = append([]string{}, args...)
	after = []string{}
	return
}
