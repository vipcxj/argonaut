package bind

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

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

const ShortDesc = "Define, bind and validate CLI arguments, then export them as shell environment variables"

const LongDesc = `Bind collects declarative argument specifications (defaults, allowed values, required/multi flags),
validates inputs, and supports two operation modes: auto, interactive and non-interactive.
It can prompt users with keyboard-driven selectors, enforce allowed values,
handle multi-valued flags, and finally emit shell-friendly "export" statements
so calling scripts can eval/source the output to import variables into their environment.`

// Run is the migrated command logic for the bind command.
func Run(cmd *cobra.Command, args []string) error {
	cmd.SilenceErrors = true
	cmd.SilenceUsage = true
	var err error
	cmdArgs, userArgs := splitAtDoubleDash(args)
	spec, err := collectSpecs(cmd, cmdArgs, userArgs)
	if err != nil {
		return err
	} else if spec == nil {
		// 仅请求帮助信息，退出成功
		return nil
	}
	realCmd := &cobra.Command{
		Use:   spec.Name,
		Short: spec.ShortDesc,
		Long:  spec.LongDesc,
		Args: func(cmd *cobra.Command, args []string) error {
			argsRange := &spec.ArgsRange
			if argsRange.LessThan(0) {
				return fmt.Errorf("invalid args range: %s", spec.ArgsRange.String())
			}
			if argsRange.IsLessThan() {
				return cobra.MaximumNArgs(argsRange.Max-1)(cmd, args)
			} else if argsRange.IsLessOrEqualThan() {
				return cobra.MaximumNArgs(argsRange.Max)(cmd, args)
			} else if argsRange.IsGreaterThan() {
				return cobra.MinimumNArgs(argsRange.Min+1)(cmd, args)
			} else if argsRange.IsGreaterOrEqualThan() {
				return cobra.MinimumNArgs(argsRange.Min)(cmd, args)
			} else if argsRange.IsSingleValue() {
				n, _ := argsRange.SingleValue()
				return cobra.ExactArgs(n)(cmd, args)
			} else if argsRange.IsUnbounded() {
				return cobra.ArbitraryArgs(cmd, args)
			} else {
				min := argsRange.Min
				if !argsRange.MinInclude {
					min += 1
				}
				max := argsRange.Max
				if !argsRange.MaxInclude {
					max -= 1
				}
				return cobra.RangeArgs(min, max)(cmd, args)
			}
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			for flagName, spec := range spec.Flags {
				valueSet := false
				if spec.Required && !cmd.Flags().Changed(flagName) {
					if spec.Default == nil {
						return fmt.Errorf("required flag %s is not provided and has no default value", flagName)
					} else {
						spec.Value = spec.Default
						valueSet = true
					}
				}
				if !valueSet {
					if spec.Multi {
						values, err := cmd.Flags().GetStringArray(flagName)
						if err != nil {
							return err
						}
						if values, err := ParseMultiValues(spec.MultiFormat, values, flagName); err != nil {
							return err
						} else {
							spec.Value = values
						}
					} else {
						value, err := cmd.Flags().GetString(flagName)
						if err != nil {
							return err
						}
						spec.Value = []string{value}
					}
				}
				if len(spec.Choices) > 0 {
					if len(spec.Value) == 0 {
						return fmt.Errorf("value for flag %s is empty but choices are defined %v", flagName, spec.Choices)
					}
					for _, val := range spec.Value {
						if !checkInStringSlice(val, spec.Choices) {
							return fmt.Errorf("value %s for flag %s is not in allowed choices %v", val, flagName, spec.Choices)
						}
					}
				}
			}
			output, err := exportEnvVar(spec)
			if err != nil {
				return err
			}
			fmt.Println(output)
			if spec.Debug {
				fmt.Fprintln(os.Stderr, output)
			}
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
		if spec.NoOptDefValue != "" {
			realCmd.Flags().Lookup(flagName).NoOptDefVal = spec.NoOptDefValue
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
		// if spec.Required {
		// 	realCmd.MarkFlagRequired(flagName)
		// }
	}
	realCmd.SetArgs(userArgs[1:]) // skip the first arg which is the command name
	return realCmd.Execute()
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
			Value:       []string{},
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

func collectSpecs(cmd *cobra.Command, bindArgs []string, userArgs []string) (*CmdSpec, error) {
	rootCmd := cmd.Root()
	specs := &CmdSpec{
		Flags:       make(map[string]*FlagSpec),
		ArgsChoices: [][]string{},
		ArgsValue:   []string{},
	}
	flagsName, err := collectFlagsName(bindArgs)
	if err != nil {
		cmd.PrintErrln(fmt.Sprintf("%s %v", cmd.ErrPrefix(), err))
		return nil, err
	}
	err = collectFlagsMulti(specs.Flags, flagsName, bindArgs)
	if err != nil {
		cmd.PrintErrln(fmt.Sprintf("%s %v", cmd.ErrPrefix(), err))
		return nil, err
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
			// interactive, err := cmd.Flags().GetBool("interactive")
			// if err != nil {
			// 	return err
			// }
			// specs.Interactive = interactive
			envPrefix, err := cmd.Flags().GetString("env-prefix")
			if err != nil {
				return err
			}
			specs.EnvPrefix = envPrefix
			debug, err := cmd.Flags().GetBool("debug")
			if err != nil {
				return err
			}
			specs.Debug = debug
			shellType, err := cmd.Flags().GetString("shell-type")
			if err != nil {
				return err
			}
			if shellType, err := ShellTypeString(shellType); err != nil {
				return fmt.Errorf("invalid shell type: %s, allowed types are: %v", shellType, ShellTypeStrings())
			} else {
				specs.ShellType = shellType
			}
			argsRangeStr, err := cmd.Flags().GetString("args-range")
			if err != nil {
				return err
			}
			if argsRange, err := NewIntRange(argsRangeStr, true); err != nil {
				return fmt.Errorf("invalid args range: %s, error: %v", argsRangeStr, err)
			} else if argsRange.LessThan(0) {
				return fmt.Errorf("invalid args range: %s, range is not valid", argsRangeStr)
			} else {
				specs.ArgsRange = argsRange
			}
			for flagName, spec := range specs.Flags {
				err = checkMultiFormat(spec.MultiFormat, flagName)
				if err != nil {
					return err
				}
				shortFlag := fmt.Sprintf("flag-%s-short", flagName)
				defaultFlag := fmt.Sprintf("flag-%s-default", flagName)
				emptyValueFlag := fmt.Sprintf("flag-%s-empty-value", flagName)
				choicesFlag := fmt.Sprintf("flag-%s-choices", flagName)
				requiredFlag := fmt.Sprintf("flag-%s-required", flagName)
				helperFlag := fmt.Sprintf("flag-%s-helper", flagName)
				envFlag := fmt.Sprintf("flag-%s-env-name", flagName)
				exportFlag := fmt.Sprintf("flag-%s-export", flagName)
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
				envValue, err := cmd.Flags().GetString(envFlag)
				if err != nil {
					return err
				}
				spec.EnvName = envValue
				exportValue, err := cmd.Flags().GetBool(exportFlag)
				if err != nil {
					return err
				}
				spec.Export = exportValue
				if cmd.Flags().Changed(defaultFlag) {
					if spec.Multi {
						if defaultValues, err := cmd.Flags().GetStringArray(defaultFlag); err != nil {
							return err
						} else {
							if defaultValues, err := ParseMultiValues(spec.MultiFormat, defaultValues, flagName); err != nil {
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
				} else {
					spec.Default = nil
				}
				emptyValue, err := cmd.Flags().GetString(emptyValueFlag)
				if err != nil {
					return err
				}
				spec.NoOptDefValue = emptyValue
				if choicesValue, err := cmd.Flags().GetStringArray(choicesFlag); err != nil {
					return err
				} else {
					if choicesValue, err := ParseMultiValues(spec.MultiFormat, choicesValue, flagName); err != nil {
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

				if len(spec.Choices) > 0 && spec.Default != nil {
					if len(spec.Default) == 0 && !spec.Required {
						return fmt.Errorf("default value for optional flag %s is empty but choices are defined %v", flagName, spec.Choices)
					}
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
	// bindCmd.Flags().BoolP("interactive", "i", false, "Enable interactive mode for user prompts")
	bindCmd.Flags().StringP("env-prefix", "e", "", "The environment variable prefix for the command; all output env vars will be prefixed with it, even those whose names are specified using --flag-<name>-env-name")
	bindCmd.Flags().BoolP("allow-repeated-flags", "r", false, "Allow repeated flag names")
	bindCmd.Flags().BoolP("debug", "d", false, "Enable debug mode, print output to stderr as well")
	bindCmd.Flags().StringP("shell-type", "", ShellTypeAuto.String(), fmt.Sprintf("The shell type for output, one of: %s", strings.Join(ShellTypeStrings(), ", ")))
	bindCmd.Flags().StringP("args-range", "a", "", "The range of positional arguments, e.g. 1, >1, <=3, [1,3], (,5], [2,), (,) for unlimited")
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
			bindCmd.Flags().StringArrayP(defaultFlag, "", []string{}, fmt.Sprintf(
				"Default values for flag %s. Note: defaults apply only when the flag is omitted; if the flag is present but given no value (e.g. '--%s'), an empty value is used instead of the default.",
				flagName, flagName,
			))
		} else {
			bindCmd.Flags().StringP(defaultFlag, "", "", fmt.Sprintf(
				"Default value for flag %s. Note: default apply only when the flag is omitted; if the flag is present but given no value (e.g. '--%s'), an empty value is used instead of the default.",
				flagName, flagName,
			))
		}
		emptyValueFlag := fmt.Sprintf("flag-%s-empty-value", flagName)
		bindCmd.Flags().StringP(emptyValueFlag, "", "", fmt.Sprintf(
			"The value to use when flag %s is present but given no explicit value (e.g. '--%s'). "+
				"Note: this applies only when the flag is provided without a value; it does not act as the default when the flag is omitted.",
			flagName, flagName,
		))
		choicesFlag := fmt.Sprintf("flag-%s-choices", flagName)
		bindCmd.Flags().StringArrayP(choicesFlag, "", []string{}, fmt.Sprintf("Allowed choices for flag %s", flagName))
		requiredFlag := fmt.Sprintf("flag-%s-required", flagName)
		bindCmd.Flags().BoolP(requiredFlag, "", false, fmt.Sprintf("Whether flag %s is required", flagName))
		envFlag := fmt.Sprintf("flag-%s-env-name", flagName)
		bindCmd.Flags().StringP(envFlag, "", "", fmt.Sprintf("Environment variable name for flag %s, default is upper-case with '-' replaced by '_'", flagName))
		exportFlag := fmt.Sprintf("flag-%s-export", flagName)
		bindCmd.Flags().BoolP(exportFlag, "", false, fmt.Sprintf("Whether flag %s should be exported as environment variable", flagName))
	}
	virtualRootCmd.AddCommand(bindCmd)
	argsWithBind := append([]string{"bind"}, bindArgs...)
	virtualRootCmd.SetArgs(argsWithBind)

	err = virtualRootCmd.Execute()
	if err != nil {
		return nil, err
	}
	// 如果只请求帮助信息，则退出成功
	if bindCmd.Flags().Changed("help") {
		return nil, nil
	}

	if len(userArgs) == 0 {
		err := errors.New("no user arguments provided after '--', at least $0 should be provided")
		bindCmd.PrintErrln(fmt.Sprintf("%s %v", bindCmd.ErrPrefix(), err))
		bindCmd.Usage()
		return nil, err
	}
	if specs.Name == "" {
		specs.Name = userArgs[0]
	}
	return specs, nil
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
