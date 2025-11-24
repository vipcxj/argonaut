package cmdtest

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"testing"

	"gopkg.in/yaml.v3"
)

// TestData 对应 YAML 文件中的单个测试用例结构
type TestData struct {
	Name        string            `yaml:"name"`
	Description string            `yaml:"description"`
	Cmd         string            `yaml:"cmd"`  // 对应 Register 的 cmd
	Args        []string          `yaml:"args"` // 参数数组，规避引号问题
	Env         map[string]string `yaml:"env"`  // 环境变量
	Expect      struct {
		Stdout   string `yaml:"stdout"`
		Stderr   string `yaml:"stderr"`
		ExitCode int    `yaml:"exitCode"`
	} `yaml:"expect"`
}

// TestGroup 对应一个 YAML 文件
type TestGroup struct {
	Name  string     // 文件名
	Tests []TestData `yaml:"tests"`
}

// TestSuite 测试套件
type TestSuite struct {
	groups   []*TestGroup
	commands map[string]func() int
	backings map[*TestGroup]*groupBacking
	mu       sync.Mutex
}

type groupBacking struct {
	path      string
	root      *yaml.Node
	testsNode *yaml.Node
	testNodes []*yaml.Node
}

// Read 读取指定目录下的所有 .yaml/.yml 文件
func Read(dir string) (*TestSuite, error) {
	suite := &TestSuite{
		groups:   make([]*TestGroup, 0),
		commands: make(map[string]func() int),
		backings: make(map[*TestGroup]*groupBacking),
	}

	err := filepath.WalkDir(dir, func(path string, d os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if d.IsDir() {
			return nil
		}

		ext := strings.ToLower(filepath.Ext(path))
		if ext != ".yaml" && ext != ".yml" {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return fmt.Errorf("read %s: %w", path, err)
		}

		var root yaml.Node
		if err := yaml.Unmarshal(content, &root); err != nil {
			return fmt.Errorf("parse %s: %w", path, err)
		}
		if len(root.Content) == 0 {
			return fmt.Errorf("%s: empty yaml", path)
		}
		doc := root.Content[0]

		testsNode, err := locateTestsNode(doc)
		if err != nil {
			return fmt.Errorf("%s: %w", path, err)
		}

		var group TestGroup
		if doc.Kind == yaml.SequenceNode {
			if err := doc.Decode(&group.Tests); err != nil {
				return fmt.Errorf("%s: decode sequence: %w", path, err)
			}
		} else {
			if err := doc.Decode(&group); err != nil {
				return fmt.Errorf("%s: decode mapping: %w", path, err)
			}
			if len(group.Tests) == 0 {
				if err := testsNode.Decode(&group.Tests); err != nil {
					return fmt.Errorf("%s: decode tests: %w", path, err)
				}
			}
		}
		group.Name = filepath.Base(path)

		if len(testsNode.Content) != len(group.Tests) {
			return fmt.Errorf("%s: tests count mismatch between yaml node and struct", path)
		}

		suite.groups = append(suite.groups, &group)
		suite.backings[&group] = &groupBacking{
			path:      path,
			root:      &root,
			testsNode: testsNode,
			testNodes: testsNode.Content,
		}
		return nil
	})
	if err != nil {
		return nil, err
	}

	return suite, nil
}

// Register 注册命令行实现
// cmd: 对应 YAML 中的 cmd 字段
// run: 执行逻辑，返回 exit code
func (s *TestSuite) Register(cmd string, run func() int) {
	s.commands[cmd] = run
}

// Run 执行测试
// 传入 *testing.T 以便集成到 go test 中
func (s *TestSuite) Run(t *testing.T) {
	s.RunWithUpdate(t, false)
}

// RunWithUpdate 执行测试；update=true 时自动写回期望
func (s *TestSuite) RunWithUpdate(t *testing.T, update bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	for _, group := range s.groups {
		g := group
		t.Run(g.Name, func(t *testing.T) {
			for i := range g.Tests {
				test := &g.Tests[i]
				name := test.Name
				if name == "" {
					name = fmt.Sprintf("Case-%d", i)
				}
				idx := i
				t.Run(name, func(t *testing.T) {
					s.runSingleTest(t, g, idx, update)
				})
			}
		})
	}
}

// runSingleTest 执行单个测试用例
func (s *TestSuite) runSingleTest(t *testing.T, group *TestGroup, idx int, update bool) {
	test := &group.Tests[idx]
	runFunc, ok := s.commands[test.Cmd]
	if !ok {
		t.Fatalf("Command '%s' not registered", test.Cmd)
	}

	// 保存现场
	oldArgs := os.Args
	oldStdout := os.Stdout
	oldStderr := os.Stderr
	type envSnapshot struct {
		value  string
		exists bool
	}
	oldEnv := make(map[string]envSnapshot)
	for k := range test.Env {
		val, exists := os.LookupEnv(k)
		oldEnv[k] = envSnapshot{value: val, exists: exists}
	}

	// 设置现场
	args := append([]string{test.Cmd}, test.Args...)
	os.Args = args
	for k, v := range test.Env {
		os.Setenv(k, v)
	}

	rOut, wOut, _ := os.Pipe()
	rErr, wErr, _ := os.Pipe()
	os.Stdout = wOut
	os.Stderr = wErr

	var exitCode int
	done := make(chan struct{}, 2)
	var gotStdout, gotStderr string

	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rOut)
		gotStdout = buf.String()
		done <- struct{}{}
	}()
	go func() {
		var buf bytes.Buffer
		_, _ = io.Copy(&buf, rErr)
		gotStderr = buf.String()
		done <- struct{}{}
	}()

	func() {
		defer func() {
			if r := recover(); r != nil {
				t.Errorf("panic: %v", r)
				exitCode = -1
			}
		}()
		exitCode = runFunc()
	}()

	// 恢复现场
	_ = wOut.Close()
	_ = wErr.Close()
	<-done
	<-done
	_ = rOut.Close()
	_ = rErr.Close()

	os.Stdout = oldStdout
	os.Stderr = oldStderr
	os.Args = oldArgs
	for k, snap := range oldEnv {
		if snap.exists {
			os.Setenv(k, snap.value)
		} else {
			os.Unsetenv(k)
		}
	}

	// 比较或更新
	changes := s.applyExpect(t, group, idx, gotStdout, gotStderr, exitCode, update)
	if update && len(changes) > 0 {
		if err := s.persistGroup(group); err != nil {
			t.Fatalf("persist %s: %v", s.backings[group].path, err)
		}
		testName := test.Name
		if testName == "" {
			testName = fmt.Sprintf("Case-%d", idx)
		}
		fmt.Printf("cmdtest: updated %s (%s): %s\n",
			s.backings[group].path, testName, strings.Join(changes, "; "))
	}
}

func (s *TestSuite) applyExpect(t *testing.T, group *TestGroup, idx int,
	stdout, stderr string, exitCode int, update bool) []string {
	test := &group.Tests[idx]
	backing := s.backings[group]
	if backing == nil {
		t.Fatalf("no yaml backing for group %s", group.Name)
	}
	testNode := backing.testNodes[idx]
	expectNode := ensureMapValue(testNode, "expect")

	var changes []string

	if exitCode != test.Expect.ExitCode {
		if update {
			test.Expect.ExitCode = exitCode
			setIntScalar(ensureMapValue(expectNode, "exitCode"), exitCode)
			changes = append(changes, fmt.Sprintf("exitCode=%d", exitCode))
		} else {
			t.Errorf("ExitCode mismatch:\nExpected: %d\nActual:   %d", test.Expect.ExitCode, exitCode)
		}
	}

	if stdout != test.Expect.Stdout {
		if update {
			test.Expect.Stdout = stdout
			setStringScalar(ensureMapValue(expectNode, "stdout"), stdout)
			changes = append(changes, fmt.Sprintf("stdout=%q", summarizeValue(stdout)))
		} else {
			t.Errorf("Stdout mismatch:\nExpected:\n%s\nActual:\n%s", test.Expect.Stdout, stdout)
		}
	}

	if stderr != test.Expect.Stderr {
		if update {
			test.Expect.Stderr = stderr
			setStringScalar(ensureMapValue(expectNode, "stderr"), stderr)
			changes = append(changes, fmt.Sprintf("stderr=%q", summarizeValue(stderr)))
		} else {
			t.Errorf("Stderr mismatch:\nExpected:\n%s\nActual:\n%s", test.Expect.Stderr, stderr)
		}
	}

	return changes
}

func (s *TestSuite) persistGroup(group *TestGroup) error {
	backing := s.backings[group]
	if backing == nil {
		return fmt.Errorf("no yaml backing for group %s", group.Name)
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(backing.root.Content[0]); err != nil {
		enc.Close()
		return err
	}
	enc.Close()
	return os.WriteFile(backing.path, buf.Bytes(), 0o644)
}

func locateTestsNode(doc *yaml.Node) (*yaml.Node, error) {
	if doc.Kind == yaml.DocumentNode && len(doc.Content) == 1 {
		doc = doc.Content[0]
	}
	switch doc.Kind {
	case yaml.MappingNode:
		for i := 0; i+1 < len(doc.Content); i += 2 {
			key := doc.Content[i]
			val := doc.Content[i+1]
			if key.Value == "tests" {
				if val.Kind != yaml.SequenceNode {
					return nil, fmt.Errorf("tests must be a sequence")
				}
				return val, nil
			}
		}
		return nil, fmt.Errorf("missing 'tests' key")
	case yaml.SequenceNode:
		return doc, nil
	default:
		return nil, fmt.Errorf("unsupported top-level yaml kind: %v", doc.Kind)
	}
}

func findMapValue(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode.Kind != yaml.MappingNode {
		return nil
	}
	for i := 0; i+1 < len(mapNode.Content); i += 2 {
		k := mapNode.Content[i]
		v := mapNode.Content[i+1]
		if k.Value == key {
			return v
		}
	}
	return nil
}

func ensureMapValue(mapNode *yaml.Node, key string) *yaml.Node {
	if mapNode.Kind != yaml.MappingNode {
		mapNode.Kind = yaml.MappingNode
		mapNode.Content = nil
	}
	if val := findMapValue(mapNode, key); val != nil {
		return val
	}
	keyNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: key}
	valNode := &yaml.Node{Kind: yaml.ScalarNode, Tag: "!!str", Value: ""}
	mapNode.Content = append(mapNode.Content, keyNode, valNode)
	return valNode
}

func setStringScalar(node *yaml.Node, val string) {
	node.Kind = yaml.ScalarNode
	node.Tag = "!!str"
	// 仅当值是单个换行（\n 或 \r\n）时使用双引号，避免 go-yaml 写成空 literal block
	if val == "\n" || val == "\r\n" {
		node.Style = yaml.DoubleQuotedStyle
	} else {
		node.Style = 0
	}
	node.Value = val
}

func setIntScalar(node *yaml.Node, val int) {
	node.Kind = yaml.ScalarNode
	node.Tag = "!!int"
	node.Style = 0
	node.Value = strconv.Itoa(val)
}

func summarizeValue(s string) string {
	s = strings.ReplaceAll(s, "\n", `\n`)
	s = strings.ReplaceAll(s, "\t", `\t`)
	if len(s) > 80 {
		return s[:77] + "..."
	}
	return s
}
