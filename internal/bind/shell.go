package bind

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

func calcEnvName(key string, default_value string, prefix string) string {
	varName := default_value
	if varName != "" {
		return varName
	}
	varName = strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
	if prefix != "" {
		varName = prefix + varName
	}
	return varName
}

// splitPreserveNewlines 将 s 拆分成若干片段，换行序列 "\r\n"、"\r"、"\n" 作为单独元素保留在结果中。
// 示例 "a\r\nb\nc\r" -> ["a", "\r\n", "b", "\n", "c", "\r"]
func splitPreserveNewlines(s string) []string {
	if s == "" {
		return []string{""}
	}
	var parts []string
	var buf strings.Builder
	for i := 0; i < len(s); {
		ch := s[i]
		if ch == '\r' || ch == '\n' {
			// flush buffer
			if buf.Len() > 0 {
				parts = append(parts, buf.String())
				buf.Reset()
			}
			// detect CRLF
			if ch == '\r' && i+1 < len(s) && s[i+1] == '\n' {
				parts = append(parts, "\r\n")
				i += 2
			} else {
				parts = append(parts, string(ch))
				i++
			}
		} else {
			buf.WriteByte(ch)
			i++
		}
	}
	if buf.Len() > 0 {
		parts = append(parts, buf.String())
	}
	return parts
}

// buildShellLiteral: POSIX shell — 最简单且可靠的策略：整体用单引号包裹。
// 如果字符串中包含单引号，用传统的 '\” 片段拼接方式（'a'\”b'）。
// 单引号内可以包含换行符，不需要额外转义。
func buildShellLiteral(s string) string {
	if s == "" {
		return "''"
	}
	// 若不含单引号，直接单引号包裹（换行也可以）
	if !strings.Contains(s, "'") {
		return "'" + s + "'"
	}
	// 含单引号时用 '\'' 片段拼接
	escaped := strings.ReplaceAll(s, "'", `'\''`)
	return "'" + escaped + "'"
}

func buildPowershellLiteral(s string) string {
	if s == "" {
		return "''"
	}
	parts := splitPreserveNewlines(s)
	var out []string
	for _, p := range parts {
		switch p {
		case "\n":
			out = append(out, "\"`n\"")
		case "\r":
			out = append(out, "\"`r\"")
		case "\r\n":
			out = append(out, "\"`r`n\"")
		default:
			// 单引号内双写单引号以转义
			if p == "" {
				out = append(out, "''")
			} else {
				out = append(out, "'"+strings.ReplaceAll(p, "'", "''")+"'")
			}
		}
	}
	// 使用 + 拼接，以产生一个可被 Invoke-Expression 正确解析的单一表达式
	return strings.Join(out, " + ")
}

// buildCmdLiteral: 保留原始换行序列，cmd 里我们使用双引号包裹片段并用 \\r \\n 文字表示换行（set/setx 中通常使用双引号）。
func buildCmdLiteral(s string) string {
	parts := splitPreserveNewlines(s)
	var out []string
	for _, p := range parts {
		switch p {
		case "\n":
			out = append(out, `"\\n"`)
		case "\r":
			out = append(out, `"\\r"`)
		case "\r\n":
			out = append(out, `"\\r\\n"`)
		default:
			// 在 cmd 里使用双引号并对内部双引号做简单转义
			escaped := strings.ReplaceAll(p, `"`, `\"`)
			if escaped == "" {
				out = append(out, `""`)
			} else {
				out = append(out, `"`+escaped+`"`)
			}
		}
	}
	return strings.Join(out, "")
}

func exportEnvVarCmdLike(varName string, val string, export bool) string {
	// buildCmdLiteral 保留原始换行并为 cmd 平台生成分段字面量
	escaped := buildCmdLiteral(val)

	if export {
		// persistent for Windows cmd: use setx
		// setx VAR "value"
		return fmt.Sprintf("setx %s \"%s\"", varName, escaped)
	} else {
		// session assignment: wrap assignment in set "VAR=value"
		// remove outer quotes if any so set "VAR=value" remains valid
		return fmt.Sprintf("set \"%s=%s\"", varName, strings.Trim(escaped, `"`))
	}
}

func exportEnvVarLinuxLike(varName string, val string, export bool) string {
	// buildShellLiteral 保留原始换行并生成 POSIX shell 字面量片段
	quoted := buildShellLiteral(val)

	if export {
		return fmt.Sprintf("export %s=%s", varName, quoted)
	} else {
		return fmt.Sprintf("%s=%s", varName, quoted)
	}
}

func escapeForPS(s string) string {
	if s == "" {
		return "''"
	}
	return "'" + strings.ReplaceAll(s, "'", "''") + "'"
}

func exportEnvVarPowershellLike(varName string, val string, export bool) string {
	// buildPowershellLiteral 返回一个可作为表达式的字符串（可能含 + 连接）
	escaped := buildPowershellLiteral(val)

	if export {
		// persistent for current user
		return fmt.Sprintf("[System.Environment]::SetEnvironmentVariable(%s,%s,'User')", escapeForPS(varName), escaped)
	} else {
		// current session
		// $Env:VAR = 'value'
		// Note: env var name in PowerShell is case-insensitive, use as-is
		return fmt.Sprintf("$Env:%s = %s", varName, escaped)
	}
}

func exportEnvVar(shellType ShellType, varName string, val string, export bool) (string, error) {
	switch shellType {
	case ShellTypeSh:
		return exportEnvVarLinuxLike(varName, val, export), nil
	case ShellTypePowershell:
		return exportEnvVarPowershellLike(varName, val, export), nil
	case ShellTypeCmd:
		return exportEnvVarCmdLike(varName, val, export), nil
	default:
		// should not reach here
		return "", fmt.Errorf("unsupported shell type: %v", shellType)
	}
}

func exportEnvVars(spec *CmdSpec) (string, error) {
	if shellType, err := decideShellType(spec.ShellType); err != nil {
		return "", err
	} else {
		if spec == nil || len(spec.Flags) == 0 {
			return "", nil
		}

		// collect keys deterministic order
		var keys []string
		for k := range spec.Flags {
			keys = append(keys, k)
		}
		sort.Strings(keys)

		var lines []string
		for _, key := range keys {
			fs := spec.Flags[key]
			if fs == nil {
				continue
			}

			// env name: prefer explicit, otherwise normalize flag key
			varName := calcEnvName(key, fs.EnvName, spec.EnvPrefix)

			val, err := OutputMultiValues(fs.MultiFormat, fs.Value)
			if err != nil {
				return "", fmt.Errorf("flag %s: %w", key, err)
			}

			if line, err := exportEnvVar(shellType, varName, val, fs.Export); err != nil {
				return "", err
			} else {
				lines = append(lines, line)
			}
		}

		return strings.Join(lines, "\n"), nil
	}
}

func decideShellType(shellType ShellType) (ShellType, error) {
	switch shellType {
	case ShellTypeSh, ShellTypePowershell, ShellTypeCmd:
		return shellType, nil
	case ShellTypeAuto:
		fallthrough
	default:
		// auto-detect
		shellName, err := detectUserShell()
		if err != nil {
			return ShellTypeAuto, fmt.Errorf("cannot detect user shell: %w", err)
		}
		shellName = strings.ToLower(shellName)
		shellName = strings.TrimSuffix(shellName, ".exe")
		switch shellName {
		case "powershell", "pwsh":
			return ShellTypePowershell, nil
		case "cmd":
			return ShellTypeCmd, nil
		default:
			// default to sh-like
			return ShellTypeSh, nil
		}
	}
}

// detectUserShell 尝试返回启动当前进程的 shell 名称（如 "bash", "zsh", "pwsh", "cmd.exe" 等）。
// 优先检查环境变量（UNIX: SHELL，Windows: COMSPEC），否则用 gopsutil 沿父进程链查找常见 shell 名称。
func detectUserShell() (string, error) {

	// 遍历父进程链寻找已知 shell
	p, err := process.NewProcess(int32(os.Getppid()))
	if err != nil {
		return "", fmt.Errorf("cannot get parent process: %w", err)
	}
	seen := map[int32]struct{}{}
	known := []string{
		"bash", "zsh", "fish", "ksh", "sh", "dash", "tcsh", "csh",
		"powershell.exe", "pwsh.exe", "cmd.exe",
	}

	for p != nil {
		if _, ok := seen[p.Pid]; ok {
			break
		}
		seen[p.Pid] = struct{}{}

		name, _ := p.Name() // 可返回短名，如 "bash" 或 "powershell.exe"
		exe, _ := p.Exe()   // 可返回完整路径

		// 归一化用于匹配
		n := strings.ToLower(name)
		if n == "" && exe != "" {
			n = strings.ToLower(filepath.Base(exe))
		}

		for _, k := range known {
			kl := strings.ToLower(k)
			if strings.Contains(n, strings.TrimSuffix(kl, ".exe")) {
				// 返回更友好的名字（优先 name，否则 exe base）
				if name != "" {
					return name, nil
				}
				return filepath.Base(exe), nil
			}
		}

		parent, perr := p.Parent()
		if perr != nil || parent == nil {
			break
		}
		p = parent
	}

	// 回退环境变量（UNIX: SHELL，Windows: COMSPEC）他们只是默认的 shell，不是当前实际使用的 shell
	if sh := os.Getenv("SHELL"); sh != "" {
		return filepath.Base(sh), nil
	}
	if com := os.Getenv("COMSPEC"); com != "" {
		return filepath.Base(com), nil
	}

	return "", fmt.Errorf("user shell not detected")
}

// ...existing code...
