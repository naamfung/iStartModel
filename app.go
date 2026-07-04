package main

import (
	"image/color"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"strings"
	"sync"
	"syscall"
	"time"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
	"fyne.io/fyne/v2/canvas"
	"fyne.io/fyne/v2/container"
	"fyne.io/fyne/v2/driver/desktop"
	"fyne.io/fyne/v2/layout"
	"fyne.io/fyne/v2/theme"
	"fyne.io/fyne/v2/widget"
	"gopkg.in/yaml.v3"
)

// SchemeConfig & Config
type SchemeConfig struct {
	Alias             string `yaml:"alias"`
	Note              string `yaml:"note"`
	Command           string `yaml:"command"`
	UseCustomEngine   bool   `yaml:"useCustomEngine"`
	UseCustomTemplate bool   `yaml:"useCustomTemplate"`
	UseCustomCommand  bool   `yaml:"useCustomCommand"` // 完全自定命令，不做任何拼接
}

type Config struct {
	LlamaServer string                  `yaml:"llamaServer"`
	Model       string                  `yaml:"model"`
	Template    string                  `yaml:"template"`
	Alias       string                  `yaml:"alias"`
	Current     string                  `yaml:"current"`
	Schemes     map[string]SchemeConfig `yaml:"schemes"`
}

func loadConfig(filename string) (*Config, error) {
	data, err := os.ReadFile(filename)
	if err != nil {
		if os.IsNotExist(err) {
			// 创建默认配置并保存
			defaultConfig := &Config{
				LlamaServer: "llama-server",
				Model:       "",
				Template:    "",
				Alias:       "",
				Current:     "",
				Schemes: map[string]SchemeConfig{
					"example": {
						Alias:             "",
						Note:              "示例方案",
						Command:           "-m model.gguf",
						UseCustomEngine:   false,
						UseCustomTemplate: false,
						UseCustomCommand:  false,
					},
				},
			}
			if err := saveConfig(filename, defaultConfig); err != nil {
				return nil, fmt.Errorf("无法创建默认配置文件: %w", err)
			}
			return defaultConfig, nil
		}
		return nil, fmt.Errorf("读取配置文件失败: %w", err)
	}
	var config Config
	if err := yaml.Unmarshal(data, &config); err != nil {
		return nil, fmt.Errorf("解析 YAML 失败: %w", err)
	}
	if len(config.Schemes) == 0 {
		return nil, fmt.Errorf("配置文件中未有定义任何 scheme")
	}
	return &config, nil
}

func saveConfig(filename string, config *Config) error {
	data, err := yaml.Marshal(config)
	if err != nil {
		return fmt.Errorf("序列化配置失败: %w", err)
	}
	return os.WriteFile(filename, data, 0644)
}

func normalizeSpaces(s string) string {
	cleaned := strings.ReplaceAll(s, "\u3000", " ")
	return strings.Join(strings.Fields(cleaned), " ")
}

func buildCommand(scheme SchemeConfig, llamaServer, model, template, globalAlias string) string {
	cmd := normalizeSpaces(scheme.Command)

	// 模式 1：完全自定命令，直接返回清洗后的命令
	if scheme.UseCustomCommand {
		return cmd
	}

	// 确定最终 alias
	alias := scheme.Alias
	if alias == "" {
		alias = globalAlias
	}

	// 模式 2：独立引擎（只换引擎，复用模型/模板）
	if scheme.UseCustomEngine {
		engine, extraArgs := cmd, ""
		if idx := strings.Index(cmd, " "); idx != -1 {
			engine = cmd[:idx]
			extraArgs = strings.TrimSpace(cmd[idx+1:])
		}

		hasModel := strings.HasPrefix(extraArgs, "-m ") || strings.HasPrefix(extraArgs, "--model ") ||
			strings.Contains(extraArgs, " -m ") || strings.Contains(extraArgs, " --model ")
		hasTemplate := strings.Contains(extraArgs, " --chat-template-file ")
		hasAlias := strings.Contains(extraArgs, " --alias ") || strings.HasPrefix(extraArgs, "--alias ")

		result := engine
		if !hasModel && model != "" {
			result += " --model " + model
		}
		if extraArgs != "" {
			result += " " + extraArgs
		}
		if !scheme.UseCustomTemplate && !hasTemplate && template != "" {
			result += " --chat-template-file " + template
		}
		if !hasAlias && alias != "" {
			result += " --alias " + alias
		}
		return result
	}

	// 模式 3：普通模式（使用全局引擎）
	hasModel := strings.HasPrefix(cmd, "-m ") || strings.HasPrefix(cmd, "--model ") ||
		strings.Contains(cmd, " -m ") || strings.Contains(cmd, " --model ")
	hasTemplate := strings.Contains(cmd, " --chat-template-file ")
	hasAlias := strings.Contains(cmd, " --alias ") || strings.HasPrefix(cmd, "--alias ")

	result := strings.TrimSpace(llamaServer)
	if !hasModel && model != "" {
		result += " --model " + model
	}
	result += " " + cmd
	if !scheme.UseCustomTemplate && !hasTemplate && template != "" {
		result += " --chat-template-file " + template
	}
	if !hasAlias && alias != "" {
		result += " --alias " + alias
	}
	return result
}

func splitCommand(cmd string) []string {
	var args []string
	var current strings.Builder
	const (
		stateNone = iota
		stateDoubleQuote
		stateSingleQuote
	)
	state := stateNone

	for _, r := range cmd {
		switch r {
		case '"':
			switch state {
			case stateNone:
				state = stateDoubleQuote
			case stateDoubleQuote:
				state = stateNone
			case stateSingleQuote:
				// 在单引号内，双引号视为普通字符
				current.WriteRune('"')
			}
		case '\'':
			switch state {
			case stateNone:
				state = stateSingleQuote
			case stateDoubleQuote:
				// 在双引号内，单引号视为普通字符
				current.WriteRune('\'')
			case stateSingleQuote:
				state = stateNone
			}
		case ' ':
			if state != stateNone {
				// 引号内的空格保留
				current.WriteRune(' ')
			} else {
				if current.Len() > 0 {
					args = append(args, current.String())
					current.Reset()
				}
			}
		default:
			current.WriteRune(r)
		}
	}
	if current.Len() > 0 {
		args = append(args, current.String())
	}
	return args
}

func printUsage() {
	fmt.Println("用法: startmodel.exe <scheme_name>")
	fmt.Println()
	fmt.Println("命令:")
	fmt.Println("  <scheme_name>    启动指定的启动方案")
	fmt.Println("  list             列出所有可用方案")
	fmt.Println("  stop <name|all>  停止某个方案或全部")
	fmt.Println("  status [name]    查看状态（不带 name 则列出全部）")
}

func printSchemes(config *Config) {
	fmt.Println("可用启动方案:")
	fmt.Println()
	for name, scheme := range config.Schemes {
		fmt.Printf("  %s", name)
		if scheme.Note != "" {
			fmt.Printf(": %s", scheme.Note)
		}
		if scheme.UseCustomEngine {
			fmt.Print(" [独立引擎]")
		}
		if scheme.UseCustomTemplate {
			fmt.Print(" [独立模板]")
		}
		if scheme.UseCustomCommand {
			fmt.Print(" [完全自定]")
		}
		fmt.Println()
	}
	fmt.Println()
}

func updateCurrentConfig(filename string, current string) error {
	data, err := os.ReadFile(filename)
	if err != nil {
		return fmt.Errorf("读取配置文件失败: %w", err)
	}
	content := string(data)
	replacer := strings.NewReplacer(
		"current: "+extractCurrent(content),
		"current: "+current,
	)
	newContent := replacer.Replace(content)
	err = os.WriteFile(filename, []byte(newContent), 0644)
	if err != nil {
		return fmt.Errorf("更新配置文件失败: %w", err)
	}
	return nil
}

func extractCurrent(content string) string {
	idx := strings.Index(content, "current:")
	if idx == -1 {
		return ""
	}
	idx += len("current:")
	end := strings.Index(content[idx:], "\n")
	if end == -1 {
		return strings.TrimSpace(content[idx:])
	}
	return strings.TrimSpace(content[idx : idx+end])
}

// ---- process management ----

var procMu sync.Mutex
var procs = make(map[string]*exec.Cmd)
var procStart = make(map[string]time.Time)

var guiMu sync.Mutex
var runningScheme string
var runningUpdateCallback func()

var stoppingMu sync.Mutex
var stoppingMap = make(map[string]bool)

var schemeValid = make(map[string]bool) // 校驗結果：scheme name -> 是否有效

func getRunningScheme() string {
	guiMu.Lock()
	defer guiMu.Unlock()
	return runningScheme
}

func setRunningScheme(name string) {
	guiMu.Lock()
	runningScheme = name
	cb := runningUpdateCallback
	guiMu.Unlock()
	if cb != nil {
		cb()
	}
}

func setRunningUpdateCallback(cb func()) {
	guiMu.Lock()
	defer guiMu.Unlock()
	runningUpdateCallback = cb
}

var errorMu sync.Mutex
var errorCallbackFunc func(string)

func setErrorCallback(cb func(string)) {
	errorMu.Lock()
	defer errorMu.Unlock()
	errorCallbackFunc = cb
}

func getErrorCallback() func(string) {
	errorMu.Lock()
	defer errorMu.Unlock()
	return errorCallbackFunc
}

func registerProcess(name string, cmd *exec.Cmd) {
	procMu.Lock()
	procs[name] = cmd
	procStart[name] = time.Now()
	procMu.Unlock()

	go func() {
		err := cmd.Wait()
		procMu.Lock()
		delete(procs, name)
		delete(procStart, name)
		procMu.Unlock()

		stoppingMu.Lock()
		_, isStopping := stoppingMap[name]
		if isStopping {
			delete(stoppingMap, name)
		}
		stoppingMu.Unlock()

		if err != nil {
			fmt.Fprintf(os.Stderr, "方案 %s 退出（err=%v）\n", name, err)
			if !isStopping {
				if cb := getErrorCallback(); cb != nil {
					cb(fmt.Sprintf("方案 %s 异常退出: %v", name, err))
				}
			}
		} else {
			fmt.Printf("方案 %s 退出\n", name)
		}
		if getRunningScheme() == name {
			setRunningScheme("")
		}
	}()
}

func stopScheme(name string) error {
	procMu.Lock()
	if name == "all" {
		names := make([]string, 0, len(procs))
		for n := range procs {
			names = append(names, n)
		}
		procMu.Unlock()
		var firstErr error
		for _, n := range names {
			if err := stopScheme(n); err != nil && firstErr == nil {
				firstErr = err
			}
		}
		setRunningScheme("")
		return firstErr
	}
	cmd, ok := procs[name]
	if !ok || cmd == nil || cmd.Process == nil {
		procMu.Unlock()
		return fmt.Errorf("方案 %s 未在运行中", name)
	}
	pid := cmd.Process.Pid
	stoppingMu.Lock()
	stoppingMap[name] = true
	stoppingMu.Unlock()
	procMu.Unlock()

	if runtime.GOOS == "windows" {
		if err := cmd.Process.Signal(os.Interrupt); err != nil {
			fmt.Fprintf(os.Stderr, "向 %s(pid=%d) 发送 Interrupt 失败: %v，改用 Kill\n", name, pid, err)
			if err2 := cmd.Process.Kill(); err2 != nil {
				return fmt.Errorf("停止 %s 失败: %w", name, err2)
			}
			fmt.Printf("已 Kill %s (pid=%d)\n", name, pid)
			return nil
		}
		fmt.Printf("已向 %s (pid=%d) 发送 Interrupt，等待最多 60s\n", name, pid)
	} else {
		if err := cmd.Process.Signal(syscall.SIGTERM); err != nil {
			fmt.Fprintf(os.Stderr, "向 %s(pid=%d) 发送 SIGTERM 失败: %v，改用 Kill\n", name, pid, err)
			if err2 := cmd.Process.Kill(); err2 != nil {
				return fmt.Errorf("停止 %s 失败: %w", name, err2)
			}
			fmt.Printf("已 Kill %s (pid=%d)\n", name, pid)
			return nil
		}
		fmt.Printf("已向 %s (pid=%d) 发送 SIGTERM，等待最多 60s\n", name, pid)
	}

	timeout := time.After(60 * time.Second)
	tick := time.NewTicker(1 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-timeout:
			procMu.Lock()
			_, still := procs[name]
			procMu.Unlock()
			if still {
				fmt.Printf("目标 %s (pid=%d) 在 60s 内未退出，准备 Kill\n", name, pid)
				if err := cmd.Process.Kill(); err != nil {
					return fmt.Errorf("Kill %s 失败: %w", name, err)
				}
				return nil
			}
			return nil
		case <-tick.C:
			procMu.Lock()
			_, still := procs[name]
			procMu.Unlock()
			if !still {
				fmt.Printf("目标 %s (pid=%d) 已退出\n", name, pid)
				return nil
			}
		}
	}
}

func statusScheme(name string) string {
	procMu.Lock()
	defer procMu.Unlock()
	if cmd, ok := procs[name]; ok && cmd != nil && cmd.Process != nil {
		start := procStart[name]
		uptime := time.Since(start).Round(time.Second)
		return fmt.Sprintf("运行中: %s  pid=%d  uptime=%s", name, cmd.Process.Pid, uptime)
	}
	return fmt.Sprintf("未运行: %s", name)
}

func statusAll() string {
	procMu.Lock()
	defer procMu.Unlock()
	if len(procs) == 0 {
		return "当前无运行中的方案"
	}
	var sb strings.Builder
	for name, cmd := range procs {
		start := procStart[name]
		uptime := time.Since(start).Round(time.Second)
		if cmd != nil && cmd.Process != nil {
			sb.WriteString(fmt.Sprintf("%s: pid=%d uptime=%s\n", name, cmd.Process.Pid, uptime))
		} else {
			sb.WriteString(fmt.Sprintf("%s: unknown process\n", name))
		}
	}
	return sb.String()
}

func truncateNote(s string, maxRunes int) string {
	runes := []rune(s)
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return s
}

// ---- hoverable ----
type hoverable struct {
	widget.BaseWidget
	child   fyne.CanvasObject
	onHover func(enter bool)
}

func newHoverable(obj fyne.CanvasObject, onHover func(enter bool)) *hoverable {
	h := &hoverable{child: obj, onHover: onHover}
	h.ExtendBaseWidget(h)
	return h
}
func (h *hoverable) CreateRenderer() fyne.WidgetRenderer {
	return widget.NewSimpleRenderer(h.child)
}
func (h *hoverable) MouseIn(ev *desktop.MouseEvent) {
	if h.onHover != nil {
		h.onHover(true)
	}
}
func (h *hoverable) MouseMoved(ev *desktop.MouseEvent) {}
func (h *hoverable) MouseOut() {
	if h.onHover != nil {
		h.onHover(false)
	}
}

// ---- 固定宽度的 Entry ----
type fixedWidthEntry struct {
	widget.Entry
	minWidth float32
}

func newFixedWidthEntry(minWidth float32) *fixedWidthEntry {
	e := &fixedWidthEntry{minWidth: minWidth}
	e.ExtendBaseWidget(e)
	return e
}

func (e *fixedWidthEntry) MinSize() fyne.Size {
	size := e.Entry.MinSize()
	size.Width = e.minWidth
	return size
}

// ---- 固定高度的多行 Entry（带垂直滚动） ----
type fixedHeightMultiLineEntry struct {
	widget.Entry
	minHeight float32
}

func newFixedHeightMultiLineEntry(minHeight float32) *fixedHeightMultiLineEntry {
	e := &fixedHeightMultiLineEntry{minHeight: minHeight}
	e.MultiLine = true
	e.Wrapping = fyne.TextWrapWord
	e.ExtendBaseWidget(e)
	return e
}

func (e *fixedHeightMultiLineEntry) MinSize() fyne.Size {
	size := e.Entry.MinSize()
	size.Height = e.minHeight
	return size
}

// 校验独立引擎的第一个参数是否为存在的可执行文件（非目录）
// 仅在独立引擎且非完全自定命令时使用
func validateCustomEngineCommand(cmdText string) error {
	cmd := normalizeSpaces(cmdText)
	if cmd == "" {
		return fmt.Errorf("独立引擎命令不能为空")
	}
	// 提取引擎路径（第一个参数）
	engine := cmd
	if idx := strings.Index(cmd, " "); idx != -1 {
		engine = cmd[:idx]
	}
	info, err := os.Stat(engine)
	if err != nil {
		return fmt.Errorf("引擎程序不存在: %s", engine)
	}
	if info.IsDir() {
		return fmt.Errorf("引擎路径是目录而非可执行文件: %s", engine)
	}
	return nil
}

// 从命令文本中提取 --chat-template-file 后面的文件路径
func extractTemplateFile(cmdText string) string {
	cmd := normalizeSpaces(cmdText)

	// 查找 --chat-template-file
	idx := strings.Index(cmd, "--chat-template-file")
	if idx == -1 {
		return ""
	}

	// 找到 --chat-template-file 后的内容
	rest := cmd[idx+len("--chat-template-file"):]
	// 跳过空格
	rest = strings.TrimLeft(rest, " ")

	// 提取路径
	if strings.HasPrefix(rest, "\"") {
		// 带双引号
		rest = rest[1:]
		endIdx := strings.Index(rest, "\"")
		if endIdx != -1 {
			return rest[:endIdx]
		}
	} else if strings.HasPrefix(rest, "'") {
		// 带单引号
		rest = rest[1:]
		endIdx := strings.Index(rest, "'")
		if endIdx != -1 {
			return rest[:endIdx]
		}
	} else {
		// 不带引号，取到下一个空格或结尾
		endIdx := strings.Index(rest, " ")
		if endIdx != -1 {
			return rest[:endIdx]
		}
		return rest
	}

	return ""
}

// 校验模板文件是否存在
func validateTemplateFile(cmdText string) error {
	cmd := normalizeSpaces(cmdText)

	// 查找 --chat-template-file
	idx := strings.Index(cmd, "--chat-template-file")
	if idx == -1 {
		return nil
	}

	templateFile := extractTemplateFile(cmdText)
	if templateFile == "" {
		return nil
	}

	info, err := os.Stat(templateFile)
	if err != nil {
		return fmt.Errorf("模板文件不存在: %s", templateFile)
	}
	if info.IsDir() {
		return fmt.Errorf("模板路径是目录而非文件: %s", templateFile)
	}

	return nil
}

// 从命令文本中提取 -m 或 --model 后面的文件路径
func extractModelFile(cmdText string) string {
	cmd := normalizeSpaces(cmdText)
	parts := strings.Fields(cmd)
	for i, part := range parts {
		if part == "-m" || part == "--model" {
			if i+1 < len(parts) {
				return parts[i+1]
			}
			return ""
		}
	}
	return ""
}

// 校验模型文件是否存在
func validateModelFile(cmdText string) error {
	modelFile := extractModelFile(cmdText)
	if modelFile == "" {
		return nil
	}

	// 去除可能的引號
	if strings.HasPrefix(modelFile, "\"") && strings.HasSuffix(modelFile, "\"") {
		modelFile = modelFile[1 : len(modelFile)-1]
	} else if strings.HasPrefix(modelFile, "'") && strings.HasSuffix(modelFile, "'") {
		modelFile = modelFile[1 : len(modelFile)-1]
	}

	if modelFile == "" {
		return nil
	}

	info, err := os.Stat(modelFile)
	if err != nil {
		return fmt.Errorf("模型文件不存在: %s", modelFile)
	}
	if info.IsDir() {
		return fmt.Errorf("模型路径是目录而非文件: %s", modelFile)
	}

	return nil
}

// 新增：自动修正 UseCustomEngine 标志
// 若 command 首段为有效引擎可执行文件，且 useCustomEngine 为 false 且非完全自定命令，
// 则自动将其设为 true 并回写配置文件。
func autoFixUseCustomEngine(name string, config *Config) {
	scheme, ok := config.Schemes[name]
	if !ok {
		return
	}
	// 完全自定命令无需修正，独立引擎已为 true 也无需修正
	if scheme.UseCustomCommand || scheme.UseCustomEngine {
		return
	}
	// 检查命令首段是否为有效引擎可执行文件
	if err := validateCustomEngineCommand(scheme.Command); err != nil {
		return
	}
	// 满足条件，修正并保存
	scheme.UseCustomEngine = true
	config.Schemes[name] = scheme
	if err := saveConfig("config.yaml", config); err != nil {
		fmt.Fprintf(os.Stderr, "自动修正 UseCustomEngine 失败: %v\n", err)
		return
	}
	fmt.Printf("自动修正方案 %s: useCustomEngine 已设为 true\n", name)
}

// ---- startScheme (CLI) ----
func startScheme(name string, config *Config) error {
	// 启动前自动修正可能遗漏的独立引擎标志
	autoFixUseCustomEngine(name, config)

	scheme, exists := config.Schemes[name]
	if !exists {
		return fmt.Errorf("方案 %s 不存在", name)
	}
	cmdStr := buildCommand(scheme, config.LlamaServer, config.Model, config.Template, config.Alias)
	parts := splitCommand(cmdStr)
	if len(parts) == 0 {
		return fmt.Errorf("方案 %s 命令为空", name)
	}

	cmd := exec.Command(parts[0], parts[1:]...)

	if runtime.GOOS == "windows" {
		cmd.SysProcAttr = &syscall.SysProcAttr{
			CreationFlags: 0x08000000,
		}
	}

	_ = os.MkdirAll("logs", 0755)
	logPath := fmt.Sprintf("logs/%s.log", name)
	f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		fmt.Fprintf(os.Stderr, "无法打开日志文件 %s: %v\n", logPath, err)
		cmd.Stdout = nil
		cmd.Stderr = nil
	} else {
		// 写入完整命令作为日志头，后跟两个空行
		_, _ = f.WriteString(cmdStr + "\n\n")
		cmd.Stdout = f
		cmd.Stderr = f
	}

	if err := cmd.Start(); err != nil {
		if f != nil {
			_ = f.Close()
		}
		return fmt.Errorf("启动失败: %w", err)
	}

	if f != nil {
		_ = f.Close()
	}

	registerProcess(name, cmd)
	fmt.Printf("启动方案 %s (pid=%d)\n命令: %s\n日志: %s\n\n", name, cmd.Process.Pid, cmdStr, logPath)

	config.Current = name
	_ = updateCurrentConfig("config.yaml", name)
	return nil
}

// ---- GUI startApp ----

func startApp(config *Config) {
	a := app.New()
	w := a.NewWindow("Start Model Manager")
	w.Resize(fyne.NewSize(700, 500))
	w.SetFixedSize(true)
	w.CenterOnScreen()

	w.SetOnClosed(func() {
		fmt.Println("窗口关闭，停止所有子进程...")
		_ = stopScheme("all")
	})

	var schemes []string
	schemeNotes := make(map[string]string)
	for name, scheme := range config.Schemes {
		schemes = append(schemes, name)
		schemeNotes[name] = scheme.Note
	}

	// ---------- 顶部容器 ----------
	topContainer := container.NewVBox()

	var errorShowing bool = false
	var errorMutex sync.Mutex

	showError := func(msg string) {
		errorMutex.Lock()
		errorShowing = true
		errorMutex.Unlock()

		topContainer.Objects = []fyne.CanvasObject{}
		errLabel := widget.NewLabel(msg)
		errLabel.Wrapping = fyne.TextWrapWord
		errLabel.TextStyle = fyne.TextStyle{Bold: true}
		errBG := canvas.NewRectangle(color.RGBA{200, 50, 50, 255})
		errContainer := container.NewMax(errBG, container.NewPadded(errLabel))
		topContainer.Add(errContainer)
		topContainer.Refresh()

		time.AfterFunc(3*time.Second, func() {
			errorMutex.Lock()
			errorShowing = false
			errorMutex.Unlock()
			topContainer.Objects = []fyne.CanvasObject{}
			topContainer.Refresh()
			if getRunningScheme() != "" {
				setRunningScheme(getRunningScheme())
			}
		})
	}
	setErrorCallback(showError)

	// ---------- 运行方案显示 ----------
	runningLabel := widget.NewLabel("")
	runningLabel.Wrapping = fyne.TextWrapWord
	runningStopBtn := widget.NewButton("停止", nil)
	runningStopBtn.Importance = widget.HighImportance

	runningContainer := container.NewVBox(
		widget.NewLabel("当前运行方案:"),
		runningLabel,
		runningStopBtn,
	)
	runningBG := canvas.NewRectangle(color.RGBA{40, 40, 50, 255})
	runningWithBG := container.NewMax(runningBG, container.NewPadded(runningContainer))

	var forceRefreshList func()

	// ---------- 列表 ----------
	var fyneList *widget.List
	fyneList = widget.NewList(
		func() int { return len(schemes) },
		func() fyne.CanvasObject {
			nameLabel := widget.NewLabel("")
			nameLabel.TextStyle = fyne.TextStyle{Bold: true}
			nameLabel.Wrapping = fyne.TextWrapWord

			noteLabel := widget.NewLabel("")
			noteLabel.Wrapping = fyne.TextWrapWord

			deleteBtn := widget.NewButton("-", nil)
			startBtn := widget.NewButton("启动", nil)

			header := container.NewHBox(deleteBtn, layout.NewSpacer())
			content := container.NewVBox(nameLabel, noteLabel)
			footer := container.NewMax(startBtn)

			itemBox := container.NewVBox(header, content, footer)
			bg := canvas.NewRectangle(theme.BackgroundColor())
			itemStack := container.NewStack(bg, itemBox)

			h := newHoverable(itemStack, func(enter bool) {
				if enter {
					bg.FillColor = color.RGBA{50, 50, 60, 255}
				} else {
					bg.FillColor = theme.BackgroundColor()
				}
				bg.Refresh()
			})
			return h
		},
		func(id int, obj fyne.CanvasObject) {
			if id < 0 || id >= len(schemes) {
				return
			}
			var stack *fyne.Container
			switch v := obj.(type) {
			case *hoverable:
				if c, ok := v.child.(*fyne.Container); ok {
					stack = c
				}
			case *fyne.Container:
				stack = v
			}
			if stack == nil || len(stack.Objects) < 2 {
				return
			}
			itemBox, ok := stack.Objects[1].(*fyne.Container)
			if !ok || len(itemBox.Objects) < 3 {
				return
			}

			var deleteBtn, startBtn *widget.Button
			var nameLabel, noteLabel *widget.Label

			if header, ok := itemBox.Objects[0].(*fyne.Container); ok {
				for _, obj := range header.Objects {
					if btn, ok := obj.(*widget.Button); ok {
						deleteBtn = btn
						break
					}
				}
			}
			if content, ok := itemBox.Objects[1].(*fyne.Container); ok {
				for i, obj := range content.Objects {
					if lbl, ok := obj.(*widget.Label); ok {
						if i == 0 {
							nameLabel = lbl
						} else if i == 1 {
							noteLabel = lbl
						}
					}
				}
			}
			if footer, ok := itemBox.Objects[2].(*fyne.Container); ok {
				for _, obj := range footer.Objects {
					if btn, ok := obj.(*widget.Button); ok {
						startBtn = btn
						break
					}
				}
			}

			schemeName := schemes[id]
			currentRunning := getRunningScheme()

			if nameLabel != nil {
				nameLabel.SetText(schemeName)
			}
			if noteLabel != nil {
				note := schemeNotes[schemeName]
				if sc, exists := config.Schemes[schemeName]; exists {
					if sc.UseCustomEngine {
						note += " [独立引擎]"
					}
					if sc.UseCustomTemplate {
						note += " [独立模板]"
					}
					if sc.UseCustomCommand {
						note += " [完全自定]"
					}
				}
				noteLabel.SetText(truncateNote(note, 68))
			}

			if startBtn != nil {
				startBtn.SetText("启动")
				if currentRunning != "" {
					startBtn.Disable()
				} else {
					startBtn.Enable()
				}

				btnId := id
				startBtn.OnTapped = func() {
					go func(btnId int) {
						name := schemes[btnId]
						// 启动前自动修正可能遗漏的独立引擎标志
						autoFixUseCustomEngine(name, config)

						sc, exists := config.Schemes[name]
						if !exists {
							fmt.Fprintf(os.Stderr, "错误: 方案 %s 不存在\n", name)
							return
						}
						cmdStr := buildCommand(sc, config.LlamaServer, config.Model, config.Template, config.Alias)
						parts := splitCommand(cmdStr)
						if len(parts) == 0 {
							fmt.Fprintf(os.Stderr, "错误: 方案 %s 命令为空\n", name)
							return
						}
						cmd := exec.Command(parts[0], parts[1:]...)
						if runtime.GOOS == "windows" {
							cmd.SysProcAttr = &syscall.SysProcAttr{
								CreationFlags: 0x08000000,
							}
						}
						_ = os.MkdirAll("logs", 0755)
						logPath := fmt.Sprintf("logs/%s.log", name)
						f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
						if err != nil {
							fmt.Fprintf(os.Stderr, "无法打开日志文件 %s: %v\n", logPath, err)
							cmd.Stdout = nil
							cmd.Stderr = nil
						} else {
							_, _ = f.WriteString(cmdStr + "\n\n")
							cmd.Stdout = f
							cmd.Stderr = f
						}
						if err := cmd.Start(); err != nil {
							if f != nil {
								_ = f.Close()
							}
							setRunningScheme("")
							if forceRefreshList != nil {
								forceRefreshList()
							}
							showError(fmt.Sprintf("启动方案 %s 失败: %v", name, err))
							return
						}
						if f != nil {
							_ = f.Close()
						}
						registerProcess(name, cmd)
						fmt.Printf("GUI 启动方案 %s (pid=%d) 日志: %s\n", name, cmd.Process.Pid, logPath)

						// 更新 current 字段并写回配置文件
						config.Current = name
						_ = updateCurrentConfig("config.yaml", config.Current)

						setRunningScheme(name)
						if forceRefreshList != nil {
							forceRefreshList()
						}
					}(btnId)
				}
			}

			if deleteBtn != nil {
				delId := id
				deleteBtn.OnTapped = func() {
					nameToRemove := schemes[delId]
					_ = stopScheme(nameToRemove)
					delete(config.Schemes, nameToRemove)
					delete(schemeNotes, nameToRemove)
					schemes = append(schemes[:delId], schemes[delId+1:]...)

					// 如果删除的是当前 current，则清空
					if config.Current == nameToRemove {
						config.Current = ""
					}
					if err := saveConfig("config.yaml", config); err != nil {
						fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
					}
					if forceRefreshList != nil {
						forceRefreshList()
					}
				}
			}

			// 根據校驗結果設置背景顏色
			if v, exists := schemeValid[schemes[id]]; exists && !v {
				if bg, ok := stack.Objects[0].(*canvas.Rectangle); ok {
					bg.FillColor = color.RGBA{255, 0, 0, 255}
					bg.Refresh()
				}
			}
		},
	)

	scroll := container.NewVScroll(fyneList)

	forceRefreshList = func() {
		if fyneList != nil {
			fyneList.Refresh()
			if scroll != nil {
				go func() {
					time.Sleep(10 * time.Millisecond)
					scroll.ScrollToBottom()
					time.Sleep(10 * time.Millisecond)
					scroll.ScrollToTop()
				}()
			}
		}
	}

	// ---------- 底部新建区域 ----------
	nameEntry := newFixedWidthEntry(175)
	nameEntry.SetPlaceHolder("方案名称")
	noteEntry := widget.NewEntry()
	noteEntry.SetPlaceHolder("备注（可选）")

	cmdEntry := newFixedHeightMultiLineEntry(100)
	cmdEntry.SetPlaceHolder("命令参数")

	customEngineCheck := widget.NewCheck("启用独立引擎", func(checked bool) {
		if checked {
			cmdEntry.SetPlaceHolder("推理引擎 + 命令参数")
		} else {
			cmdEntry.SetPlaceHolder("命令参数")
		}
	})
	customEngineCheck.SetChecked(false)

	customTemplateCheck := widget.NewCheck("启用独立模板", func(checked bool) {})
	customTemplateCheck.SetChecked(false)

	// 完全自定命令复选框：勾选时重置并禁用独立引擎/独立模板，取消时恢复可用状态
	customCommandCheck := widget.NewCheck("完全自定命令", func(checked bool) {
		if checked {
			customEngineCheck.SetChecked(false)
			customEngineCheck.Disable()
			customTemplateCheck.SetChecked(false)
			customTemplateCheck.Disable()
			cmdEntry.SetPlaceHolder("完整命令（将使用此处定义的完整命令，而不对参数作任何处理）")
		} else {
			customEngineCheck.Enable()
			customTemplateCheck.Enable()
			cmdEntry.SetPlaceHolder("命令参数")
		}
	})
	customCommandCheck.SetChecked(false)

	rightBox := container.NewVBox(customEngineCheck, customTemplateCheck, customCommandCheck)
	cmdRow := container.NewBorder(nil, nil, nil, rightBox, cmdEntry)

	nameRow := container.NewBorder(nil, nil, nameEntry, nil, noteEntry)

	bottomContainer := container.NewVBox(
		nameRow,
		cmdRow,
	)
	bottomBG := canvas.NewRectangle(color.RGBA{30, 30, 30, 255})
	bottomWithBG := container.NewMax(bottomBG, container.NewPadded(bottomContainer))
	bottomHolder := container.NewVBox(bottomWithBG)
	bottomHolder.Hide()

	newBtn := widget.NewButton("新建", func() {
		name := strings.TrimSpace(nameEntry.Text)
		if name == "" {
			return
		}

		cleanCommand := normalizeSpaces(cmdEntry.Text)
		cleanNote := normalizeSpaces(noteEntry.Text)

		useCustomEngine := customEngineCheck.Checked
		useCustomTemplate := customTemplateCheck.Checked
		useCustomCommand := customCommandCheck.Checked

		// 校验独立引擎（且不是完全自定命令模式）
		if useCustomEngine && !useCustomCommand {
			if err := validateCustomEngineCommand(cleanCommand); err != nil {
				showError(fmt.Sprintf("独立引擎校验失败: %v", err))
				return
			}
		}

		// 保存配置（浅拷贝 schemes map）
		newConfig := *config
		newConfig.Schemes = make(map[string]SchemeConfig)
		for k, v := range config.Schemes {
			newConfig.Schemes[k] = v
		}
		newConfig.Schemes[name] = SchemeConfig{
			Alias:             config.Alias,
			Note:              cleanNote,
			Command:           cleanCommand,
			UseCustomEngine:   useCustomEngine,
			UseCustomTemplate: useCustomTemplate,
			UseCustomCommand:  useCustomCommand,
		}

		if err := saveConfig("config.yaml", &newConfig); err != nil {
			fmt.Fprintf(os.Stderr, "保存配置失败: %v\n", err)
			showError(fmt.Sprintf("保存失败: %v", err))
			return
		}

		// 保存成功，更新内存变量
		config.Schemes = newConfig.Schemes
		schemeNotes[name] = cleanNote
		schemes = append(schemes, name)

		if forceRefreshList != nil {
			forceRefreshList()
		}

		// 清空输入框
		nameEntry.SetText("")
		noteEntry.SetText("")
		cmdEntry.SetText("")
		customEngineCheck.SetChecked(false)
		customTemplateCheck.SetChecked(false)
		customCommandCheck.SetChecked(false)
		// 确保独立引擎/独立模板恢复可用（防止残留禁用状态）
		customEngineCheck.Enable()
		customTemplateCheck.Enable()
		cmdEntry.SetPlaceHolder("命令参数")

		// 自动隐藏新建区域
		bottomHolder.Hide()
		bottomHolder.Refresh()
	})

	bottomContainer.Objects = append(bottomContainer.Objects, newBtn)

	// ========== 工具栏（使用内置刷新图标，宽度一致） ==========
	reloadBtn := widget.NewButtonWithIcon("", theme.ViewRefreshIcon(), func() {
		newConfig, err := loadConfig("config.yaml")
		if err != nil {
			showError(fmt.Sprintf("重新加载配置失败: %v", err))
			return
		}

		// 校驗每個 scheme 的引擎路徑、模板文件和模型文件
		schemeValid = make(map[string]bool)
		for name, sc := range newConfig.Schemes {
			isValid := true

			// 校驗引擎路徑
			if sc.UseCustomEngine && !sc.UseCustomCommand && sc.Command != "" {
				if err := validateCustomEngineCommand(sc.Command); err != nil {
					isValid = false
					fmt.Printf("方案 %s 引擎路徑無效: %v\n", name, err)
				}
			}

			// 校驗模板文件
			if sc.Command != "" {
				hasTemplate := strings.Contains(sc.Command, "--chat-template-file")
				if hasTemplate {
					if err := validateTemplateFile(sc.Command); err != nil {
						isValid = false
						fmt.Printf("方案 %s 模板文件無效: %v\n", name, err)
					}
				}
			}

			// 校驗模型文件
			if sc.Command != "" {
				if err := validateModelFile(sc.Command); err != nil {
					isValid = false
					fmt.Printf("方案 %s 模型文件無效: %v\n", name, err)
				}
			}

			schemeValid[name] = isValid
			newConfig.Schemes[name] = sc
		}

		*config = *newConfig

		schemes = nil
		schemeNotes = make(map[string]string)
		for name, sc := range config.Schemes {
			schemes = append(schemes, name)
			schemeNotes[name] = sc.Note
		}

		if forceRefreshList != nil {
			forceRefreshList()
		}
		if getRunningScheme() != "" {
			setRunningScheme(getRunningScheme())
		}
	})
	reloadBtn.Importance = widget.LowImportance // 扁平化外观，与右侧工具栏一致

	// 启动当前方案按钮
	startCurrentBtn := widget.NewButtonWithIcon("", theme.MediaPlayIcon(), func() {
		if config.Current != "" && getRunningScheme() == "" {
			go func() {
				schemeName := config.Current
				// 启动前自动修正可能遗漏的独立引擎标志
				autoFixUseCustomEngine(schemeName, config)

				scheme, exists := config.Schemes[schemeName]
				if !exists {
					fmt.Fprintf(os.Stderr, "错误: 方案 %s 不存在\n", schemeName)
					return
				}
				cmdStr := buildCommand(scheme, config.LlamaServer, config.Model, config.Template, config.Alias)
				parts := splitCommand(cmdStr)
				if len(parts) == 0 {
					fmt.Fprintf(os.Stderr, "错误: 方案 %s 命令为空\n", schemeName)
					return
				}

				cmd := exec.Command(parts[0], parts[1:]...)
				if runtime.GOOS == "windows" {
					cmd.SysProcAttr = &syscall.SysProcAttr{
						CreationFlags: 0x08000000,
					}
				}
				_ = os.MkdirAll("logs", 0755)
				logPath := fmt.Sprintf("logs/%s.log", schemeName)
				f, err := os.OpenFile(logPath, os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
				if err != nil {
					fmt.Fprintf(os.Stderr, "无法打开日志文件 %s: %v\n", logPath, err)
					cmd.Stdout = nil
					cmd.Stderr = nil
				} else {
					_, _ = f.WriteString(cmdStr + "\n\n")
					cmd.Stdout = f
					cmd.Stderr = f
				}
				if err := cmd.Start(); err != nil {
					if f != nil {
						_ = f.Close()
					}
					setRunningScheme("")
					if forceRefreshList != nil {
						forceRefreshList()
					}
					showError(fmt.Sprintf("启动方案 %s 失败: %v", schemeName, err))
					return
				}
				if f != nil {
					_ = f.Close()
				}
				registerProcess(schemeName, cmd)
				fmt.Printf("GUI 启动方案 %s (pid=%d) 日志: %s\n", schemeName, cmd.Process.Pid, logPath)

				config.Current = schemeName
				_ = updateCurrentConfig("config.yaml", config.Current)

				setRunningScheme(schemeName)
				if forceRefreshList != nil {
					forceRefreshList()
				}
			}()
		}
	})
	startCurrentBtn.Importance = widget.LowImportance // 扁平化外观，与右侧工具栏一致
	// 默认隐藏，当 config.Current 有值时显示
	startCurrentBtn.Hide()

	// 右侧工具栏（原有 + 按钮 + 启动当前方案按钮）
	toolbar := widget.NewToolbar(
		widget.NewToolbarAction(theme.ContentAddIcon(), func() {
			if bottomHolder.Visible() {
				bottomHolder.Hide()
			} else {
				bottomHolder.Show()
			}
			bottomHolder.Refresh()
		}),
		widget.NewToolbarAction(theme.MediaPlayIcon(), func() {
			if config.Current != "" && getRunningScheme() == "" {
				startCurrentBtn.OnTapped()
			}
		}),
	)

	// 顶部栏：左侧重载按钮，右侧工具栏
	topBar := container.NewBorder(nil, nil,
		container.NewHBox(reloadBtn),
		container.NewHBox(layout.NewSpacer(), toolbar),
	)

	// ---------- 运行状态更新回调 ----------
	setRunningUpdateCallback(func() {
		errorMutex.Lock()
		showing := errorShowing
		errorMutex.Unlock()
		if showing {
			if forceRefreshList != nil {
				forceRefreshList()
			}
			return
		}
		running := getRunningScheme()
		if running == "" {
			if len(topContainer.Objects) > 0 {
				topContainer.Objects = []fyne.CanvasObject{}
				topContainer.Refresh()
			}
			// 显示/隐藏启动当前方案按钮
			if config.Current != "" {
				startCurrentBtn.Show()
			} else {
				startCurrentBtn.Hide()
			}
		} else {
			if len(topContainer.Objects) == 0 || topContainer.Objects[0] != runningWithBG {
				topContainer.Objects = []fyne.CanvasObject{runningWithBG}
				topContainer.Refresh()
			}
			runningLabel.SetText(running)
			runningStopBtn.OnTapped = func() {
				if err := stopScheme(running); err != nil {
					fmt.Fprintf(os.Stderr, "停止失败: %v\n", err)
				}
			}
			// 运行中时隐藏启动当前方案按钮
			startCurrentBtn.Hide()
		}
		if forceRefreshList != nil {
			forceRefreshList()
		}
	})

	// ---------- 信号处理 ----------
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	go func() {
		<-sigChan
		fmt.Println("收到终止信号，停止所有子进程...")
		_ = stopScheme("all")
		os.Exit(0)
	}()

	// ---------- 顶层布局 ----------
	mainContent := container.NewBorder(topContainer, nil, nil, nil, scroll)
	content := container.NewBorder(topBar, bottomHolder, nil, nil, mainContent)
	w.SetContent(content)
	w.ShowAndRun()
}

// ---- main ----

func main() {
	if len(os.Args) < 2 {
		config, err := loadConfig("config.yaml")
		if err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
		if len(config.Schemes) == 0 {
			fmt.Fprintf(os.Stderr, "错误: 配置文件中未有定义任何 scheme\n")
			os.Exit(1)
		}
		startApp(config)
		return
	}

	config, err := loadConfig("config.yaml")
	if err != nil {
		fmt.Fprintf(os.Stderr, "错误: %v\n", err)
		os.Exit(1)
	}

	command := os.Args[1]
	switch command {
	case "list":
		printSchemes(config)
	case "stop":
		if len(os.Args) >= 3 {
			target := os.Args[2]
			if err := stopScheme(target); err != nil {
				fmt.Fprintf(os.Stderr, "停止失败: %v\n", err)
				os.Exit(1)
			}
		} else {
			fmt.Println("用法: stop <name|all>")
			os.Exit(1)
		}
	case "status":
		if len(os.Args) >= 3 {
			fmt.Println(statusScheme(os.Args[2]))
		} else {
			fmt.Println(statusAll())
		}
	default:
		if _, exists := config.Schemes[command]; !exists {
			if config.Current != "" {
				command = config.Current
			} else {
				printUsage()
				os.Exit(0)
			}
		}
		if err := startScheme(command, config); err != nil {
			fmt.Fprintf(os.Stderr, "错误: %v\n", err)
			os.Exit(1)
		}
	}
}