package bind

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

type ArgSpec struct {
	ShortName   string
	Default     []string
	Choices     []string
	Required    bool
	Multi       bool
	MultiFormat string
	Helper      string
}

var AllowedMultiFormats = []string{"comma", "newline", "space", "json"}

func checkMultiFormat(format string) error {
	for _, f := range AllowedMultiFormats {
		if format == f {
			return nil
		}
	}
	return fmt.Errorf("invalid multi format: %s, allowed formats are: %v", format, AllowedMultiFormats)
}

type CmdSpec struct {
	Name        string
	ShortDesc   string
	LongDesc    string
	Interactive bool
	Args        map[string]*ArgSpec
}

const ShortDesc = "Define, bind and validate CLI arguments, then export them as shell environment variables"

const LongDesc = `Bind collects declarative argument specifications (defaults, allowed values, required/multi flags),
validates inputs, and supports two operation modes: auto, interactive and non-interactive.
It can prompt users with keyboard-driven selectors, enforce allowed values,
handle multi-valued arguments, and finally emit shell-friendly "export" statements
so calling scripts can eval/source the output to import variables into their environment.`

// Run is the migrated command logic for the bind command.
func Run(cmd *cobra.Command, args []string) error {
	var err error
	cmdArgs, userArgs := splitAtDoubleDash(args)
	if len(userArgs) == 0 {
		return fmt.Errorf("no user arguments provided after '--', at least $0 should be provided")
	}
	spec, err := collectSpecs(cmdArgs)
	if err != nil {
		return err
	}
	if spec.Name == "" {
		spec.Name = userArgs[0]
	}
	realCmd := &cobra.Command{
		Use:   spec.Name,
		Short: spec.ShortDesc,
		Long:  spec.LongDesc,
		RunE: func(cmd *cobra.Command, args []string) error {
			return nil
		},
	}

	for argName, spec := range spec.Args {
		if !spec.Multi {
			var defaultVar string
			if len(spec.Default) > 0 {
				defaultVar = spec.Default[0]
			} else {
				defaultVar = ""
			}
			realCmd.Flags().StringP(argName, spec.ShortName, defaultVar, spec.Helper)
		} else {
			realCmd.Flags().StringSliceP(argName, spec.ShortName, spec.Default, spec.Helper)
		}
	}
	realCmd.SetArgs(userArgs[1:]) // skip the first arg which is the command name
	err = realCmd.Execute()
	if err != nil {
		return err
	}
	return nil
}

func collectArgsName(args []string) ([]string, error) {
	fs := pflag.NewFlagSet("args", pflag.ContinueOnError)
	fs.StringSliceP("arg", "a", []string{}, "")
	fs.Parse(args)
	return fs.GetStringSlice("arg")
}

func getRepeatedArgsName(names []string) []string {
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

func collectArgsMulti(specs map[string]*ArgSpec, argsName []string, argsValues []string) error {
	fs := pflag.NewFlagSet("args", pflag.ContinueOnError)
	for _, argName := range argsName {
		_, exists := specs[argName]
		if exists {
			continue
		}
		specs[argName] = &ArgSpec{}
		arg_name := fmt.Sprintf("arg-%s-multi", argName)
		fs.BoolP(arg_name, "", false, "")
		arg_name = fmt.Sprintf("arg-%s-multi-format", argName)
		fs.StringP(arg_name, "", "", "")
	}
	fs.Parse(argsValues)
	for _, argName := range argsName {
		arg_name := fmt.Sprintf("arg-%s-multi", argName)
		multi, err := fs.GetBool(arg_name)
		if err != nil {
			return err
		}
		specs[argName].Multi = multi
		arg_name = fmt.Sprintf("arg-%s-multi-format", argName)
		multiFormat, err := fs.GetString(arg_name)
		if err != nil {
			return err
		}
		specs[argName].MultiFormat = multiFormat
	}
	return nil
}

func collectSpecs(args []string) (*CmdSpec, error) {
	specs := &CmdSpec{
		Args: make(map[string]*ArgSpec),
	}
	argsName, err := collectArgsName(args)
	if err != nil {
		return nil, err
	}
	err = collectArgsMulti(specs.Args, argsName, args)
	if err != nil {
		return nil, err
	}
	bindCmd := &cobra.Command{
		Use:   "bind",
		Short: ShortDesc,
		Long:  LongDesc,
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
			allowRepeated, err := cmd.Flags().GetBool("allow-repeated-args")
			if err != nil {
				return err
			}
			if !allowRepeated {
				repeated := getRepeatedArgsName(argsName)
				if len(repeated) > 0 {
					return fmt.Errorf("repeated argument names: %v", repeated)
				}
			}
			interactive, err := cmd.Flags().GetBool("interactive")
			if err != nil {
				return err
			}
			specs.Interactive = interactive
			for argName, spec := range specs.Args {
				err = checkMultiFormat(spec.MultiFormat)
				if err != nil {
					return err
				}
				shortFlag := fmt.Sprintf("arg-%s-short", argName)
				defaultFlag := fmt.Sprintf("arg-%s-default", argName)
				choicesFlag := fmt.Sprintf("arg-%s-choices", argName)
				requiredFlag := fmt.Sprintf("arg-%s-required", argName)
				helperFlag := fmt.Sprintf("arg-%s-helper", argName)
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
				if cmd.Flags().Changed(defaultFlag) {
					if spec.Multi {
						defaultValues, err := cmd.Flags().GetStringSlice(defaultFlag)
						if err != nil {
							return err
						}
						spec.Default = defaultValues
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
				choicesValue, err := cmd.Flags().GetStringSlice(choicesFlag)
				if err != nil {
					return err
				}
				spec.Choices = choicesValue
				requiredValue, err := cmd.Flags().GetBool(requiredFlag)
				if err != nil {
					return err
				}
				spec.Required = requiredValue
			}
			return nil
		},
	}
	bindCmd.Flags().StringP("name", "n", "", "The name of the command")
	bindCmd.Flags().StringP("short", "s", "", "The short description of the command")
	bindCmd.Flags().StringP("long", "l", "", "The long description of the command")
	bindCmd.Flags().BoolP("interactive", "i", false, "Enable interactive mode for user prompts")
	bindCmd.Flags().BoolP("allow-repeated-args", "r", false, "Allow repeated argument names")
	bindCmd.Flags().StringSliceP("arg", "a", nil, "Name For argument")
	for argName, spec := range specs.Args {
		shortFlag := fmt.Sprintf("arg-%s-short", argName)
		bindCmd.Flags().StringP(shortFlag, "", "", fmt.Sprintf("Short name for argument %s", argName))
		helpFlag := fmt.Sprintf("arg-%s-helper", argName)
		bindCmd.Flags().StringP(helpFlag, "", "", fmt.Sprintf("Helper text for argument %s", argName))
		multiFlag := fmt.Sprintf("arg-%s-multi", argName)
		bindCmd.Flags().BoolP(multiFlag, "", false, fmt.Sprintf("Whether argument %s is multi-valued", argName))
		multiFormatFlag := fmt.Sprintf("arg-%s-multi-format", argName)
		bindCmd.Flags().StringP(multiFormatFlag, "", AllowedMultiFormats[0], fmt.Sprintf("Multi value format for argument %s, (%v)", argName, AllowedMultiFormats))
		defaultFlag := fmt.Sprintf("arg-%s-default", argName)
		if spec.Multi {
			bindCmd.Flags().StringSliceP(defaultFlag, "", []string{}, fmt.Sprintf("Default values for argument %s", argName))
		} else {
			bindCmd.Flags().StringP(defaultFlag, "", "", fmt.Sprintf("Default value for argument %s", argName))
		}
		choicesFlag := fmt.Sprintf("arg-%s-choices", argName)
		bindCmd.Flags().StringSliceP(choicesFlag, "", []string{}, fmt.Sprintf("Allowed choices for argument %s", argName))
		requiredFlag := fmt.Sprintf("arg-%s-required", argName)
		bindCmd.Flags().BoolP(requiredFlag, "", false, fmt.Sprintf("Whether argument %s is required", argName))
	}
	bindCmd.SetArgs(args)
	err = bindCmd.Execute()
	if err != nil {
		os.Exit(1)
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
