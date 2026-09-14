package service

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	goruntime "runtime"
	"time"

	"buddyswitch/global"
	"buddyswitch/plugin"
	"buddyswitch/store"
	"buddyswitch/upstream"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// App 是暴露给前端的 Wails 绑定对象。
type App struct {
	ctx       context.Context
	reg       *plugin.Registry
	state     *store.Store
	upstreams *upstream.Catalog
}

// NewApp 装配插件注册表、状态存储与上游目录。必须在 global.Init() 之后调用。
func NewApp() *App {
	app := &App{
		reg:       plugin.NewRegistry(global.Config().PluginDir),
		state:     store.Open(global.StatePath()),
		upstreams: upstream.Open(global.UpstreamPath()),
	}
	if err := app.reg.Load(); err != nil {
		global.Log.Errorf("加载插件失败: %v", err)
	} else {
		global.Log.Infof("已加载 %d 个插件", len(app.reg.List()))
	}
	a := app.upstreams.List()
	if len(a) > 0 {
		global.Log.Infof("上游目录已有 %d 个上游", len(a))
	}
	return app
}

// Startup 实现 Wails 的启动回调。
func (a *App) Startup(ctx context.Context) {
	a.ctx = ctx
}

// ---------------------------------------------------------------------------
// 窗口控制
// ---------------------------------------------------------------------------

func (a *App) WindowMinimise() {
	runtime.WindowMinimise(a.ctx)
}

func (a *App) WindowToggleMaximise() {
	runtime.WindowToggleMaximise(a.ctx)
}

func (a *App) WindowClose() {
	runtime.Quit(a.ctx)
}

// ---------------------------------------------------------------------------
// 系统信息
// ---------------------------------------------------------------------------

// SystemInfo 描述运行环境。
type SystemInfo struct {
	OS          string `json:"os"`
	Arch        string `json:"arch"`
	NumCPU      int    `json:"num_cpu"`
	Hostname    string `json:"hostname"`
	GoVer       string `json:"go_ver"`
	Time        string `json:"time"`
	ProcessName string `json:"process_name"`
	WorkDir     string `json:"work_dir"`
}

func (a *App) GetSystemInfo() SystemInfo {
	hostname, _ := os.Hostname()
	wd, _ := os.Getwd()
	return SystemInfo{
		OS:          goruntime.GOOS,
		Arch:        goruntime.GOARCH,
		NumCPU:      goruntime.NumCPU(),
		Hostname:    hostname,
		GoVer:       goruntime.Version(),
		Time:        time.Now().Format(time.DateTime),
		ProcessName: global.GetProcessName(),
		WorkDir:     wd,
	}
}

func (a *App) GetProcessName() string {
	return global.GetProcessName()
}

// ---------------------------------------------------------------------------
// 文件对话框与读写
// ---------------------------------------------------------------------------

func (a *App) OpenFileSelect() string {
	file, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择模型配置文件",
		Filters: []runtime.FileFilter{
			{DisplayName: "JSON 文件", Pattern: "*.json"},
			{DisplayName: "所有文件", Pattern: "*.*"},
		},
	})
	if err != nil {
		global.Log.Warnf("打开文件对话框失败: %v", err)
		return ""
	}
	return file
}

func (a *App) OpenFolderSelect() string {
	folder, err := runtime.OpenDirectoryDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "选择目录",
	})
	if err != nil {
		global.Log.Warnf("打开目录对话框失败: %v", err)
		return ""
	}
	return folder
}

func (a *App) ReadFileContent(path string) string {
	if path == "" {
		return ""
	}
	data, err := os.ReadFile(path)
	if err != nil {
		global.Log.Warnf("读取文件失败: %v", err)
		return fmt.Sprintf("读取失败: %v", err)
	}
	return string(data)
}

// Notify 向前端推送一条通知事件。
func (a *App) Notify(title string, message string) {
	runtime.EventsEmit(a.ctx, "notification", map[string]string{
		"title":   title,
		"message": message,
	})
}

// revealPath 在系统文件管理器中定位路径（目录则直接打开）。
func revealPath(path string) error {
	switch goruntime.GOOS {
	case "windows":
		return exec.Command("explorer", path).Start()
	case "darwin":
		return exec.Command("open", path).Start()
	default:
		return exec.Command("xdg-open", path).Start()
	}
}
