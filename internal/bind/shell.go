package bind

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/shirou/gopsutil/v4/process"
)

func exportEnvVarCmdLike(spec *CmdSpec) (string, error) {
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
		varName := fs.EnvName
		if varName == "" {
			varName = strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		}

		val, err := outputMultiValues(fs.MultiFormat, fs.Value)
		if err != nil {
			return "", fmt.Errorf("flag %s: %w", key, err)
		}

		// prepare value for cmd
		// minimal escaping: double quotes inside value replaced with `\"` (best-effort)
		escaped := strings.ReplaceAll(val, `"`, `\"`)

		if fs.Export {
			// persistent for Windows cmd: use setx
			// setx VAR "value"
			lines = append(lines, fmt.Sprintf("setx %s \"%s\"", varName, escaped))
		} else {
			// set for current cmd session: set "VAR=value"
			// quoting the whole assignment avoids issues with spaces
			lines = append(lines, fmt.Sprintf("set \"%s=%s\"", varName, escaped))
		}
	}

	return strings.Join(lines, "\r\n"), nil
}

func exportEnvVarLinuxLike(spec *CmdSpec) (string, error) {
	if spec == nil || len(spec.Flags) == 0 {
		return "", nil
	}

	var keys []string
	for k := range spec.Flags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	escapeSingle := func(s string) string {
		// escape single quote for POSIX shell: ' -> '\''  (close, insert quoted single quote, reopen)
		// implement by replacing ' with '\'' sequence
		if s == "" {
			return "''"
		}
		// replace each ' with '\'' pattern
		return "'" + strings.ReplaceAll(s, "'", `'\''`) + "'"
	}

	var lines []string
	for _, key := range keys {
		fs := spec.Flags[key]
		if fs == nil {
			continue
		}

		varName := fs.EnvName
		if varName == "" {
			varName = strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		}

		val, err := outputMultiValues(fs.MultiFormat, fs.Value)
		if err != nil {
			return "", fmt.Errorf("flag %s: %w", key, err)
		}

		quoted := escapeSingle(val)

		if fs.Export {
			lines = append(lines, fmt.Sprintf("export %s=%s", varName, quoted))
		} else {
			lines = append(lines, fmt.Sprintf("%s=%s", varName, quoted))
		}
	}

	return strings.Join(lines, "\n"), nil
}

func exportEnvVarPowershellLike(spec *CmdSpec) (string, error) {
	if spec == nil || len(spec.Flags) == 0 {
		return "", nil
	}

	var keys []string
	for k := range spec.Flags {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// for PowerShell use single-quoted literal strings; single quote inside is represented by doubling it.
	escapeForPS := func(s string) string {
		if s == "" {
			return "''"
		}
		return "'" + strings.ReplaceAll(s, "'", "''") + "'"
	}

	var lines []string
	for _, key := range keys {
		fs := spec.Flags[key]
		if fs == nil {
			continue
		}

		varName := fs.EnvName
		if varName == "" {
			varName = strings.ToUpper(strings.ReplaceAll(key, "-", "_"))
		}

		val, err := outputMultiValues(fs.MultiFormat, fs.Value)
		if err != nil {
			return "", fmt.Errorf("flag %s: %w", key, err)
		}

		escaped := escapeForPS(val)

		if fs.Export {
			// persistent for current user
			lines = append(lines, fmt.Sprintf("[System.Environment]::SetEnvironmentVariable(%s,%s,'User')", escapeForPS(varName), escaped))
		} else {
			// current session
			// $Env:VAR = 'value'
			// Note: env var name in PowerShell is case-insensitive, use as-is
			lines = append(lines, fmt.Sprintf("$Env:%s = %s", varName, escaped))
		}
	}

	return strings.Join(lines, "\n"), nil
}

func exportEnvVar(spec *CmdSpec) (string, error) {
	switch spec.ShellType {
	case ShellTypeSh:
		return exportEnvVarLinuxLike(spec)
	case ShellTypePowershell:
		return exportEnvVarPowershellLike(spec)
	case ShellTypeCmd:
		return exportEnvVarCmdLike(spec)
	case ShellTypeAuto:
		fallthrough
	default:
		// auto-detect
		shellName, err := detectUserShell()
		if err != nil {
			// default to sh-like
			return "", fmt.Errorf("cannot detect user shell: %w", err)
		}
		lowerName := strings.ToLower(shellName)
		if strings.Contains(lowerName, "powershell") || strings.Contains(lowerName, "pwsh") {
			return exportEnvVarPowershellLike(spec)
		} else if strings.Contains(lowerName, "cmd.exe") || strings.Contains(lowerName, "cmd") {
			return exportEnvVarCmdLike(spec)
		} else {
			// default to sh-like
			return exportEnvVarLinuxLike(spec)
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
