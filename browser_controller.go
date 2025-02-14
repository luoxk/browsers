package browsers

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/fetch"
	"github.com/chromedp/chromedp"
	"image"
	"log"
	"sync"
)

// BrowserOptions 用于配置浏览器启动参数
type BrowserOptions struct {
	Path        string                                            // 浏览器启动路径
	Fingerprint string                                            // 指纹参数
	Proxy       string                                            // 代理地址
	UserDir     string                                            // 用户目录
	Headless    bool                                              // 是否启用无头模式
	HookFunc    func(ctx context.Context) func(event interface{}) // 网络拦截器
	WindowSize  *image.Point                                      //窗口大小
	DisableGPU  bool                                              //禁用硬件加速
}

// BrowserController 用于管理多个浏览器实例
type BrowserController struct {
	instances map[int]*BrowserInstance // 浏览器实例的映射
	nextID    int                      // 下一个浏览器实例的 ID
	mu        sync.Mutex               // 用于保护 instances 和 nextID 的互斥锁
}

// NewBrowserController 创建一个新的 BrowserController 实例
func NewBrowserController() *BrowserController {
	return &BrowserController{
		instances: make(map[int]*BrowserInstance),
		nextID:    1, // 从 1 开始分配 ID
	}
}

// LaunchBrowser 启动一个新的浏览器实例
func (bc *BrowserController) LaunchBrowser(options BrowserOptions) (*BrowserInstance, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// 配置浏览器启动参数
	allocatorOpts := append(
		chromedp.DefaultExecAllocatorOptions[:],
		chromedp.ExecPath(options.Path),             // 指定浏览器路径
		chromedp.Flag("headless", options.Headless), // 是否启用无头模式
	)
	if options.UserDir != "" {
		allocatorOpts = append(allocatorOpts, chromedp.UserDataDir(options.UserDir))
	}

	if options.DisableGPU {
		allocatorOpts = append(allocatorOpts, chromedp.DisableGPU)
	}

	if options.WindowSize != nil {
		allocatorOpts = append(allocatorOpts, chromedp.WindowSize(options.WindowSize.X, options.WindowSize.Y))
	}

	// 设置代理
	if options.Proxy != "" {
		allocatorOpts = append(allocatorOpts, chromedp.ProxyServer(options.Proxy))
	}

	// 设置指纹参数
	if options.Fingerprint != "" {
		allocatorOpts = append(allocatorOpts, chromedp.Flag("fp", options.Fingerprint))
	}

	// 创建上下文
	allocCtx, cancelAlloc := chromedp.NewExecAllocator(context.Background(), allocatorOpts...)
	ctx, cancel := chromedp.NewContext(allocCtx)

	// 启动浏览器
	err := chromedp.Run(ctx, chromedp.Navigate("about:blank"))
	if err != nil {
		cancel()
		cancelAlloc()
		return nil, err
	}

	// 获取浏览器实例
	browser := chromedp.FromContext(ctx)
	// 设置网络拦截器
	if options.HookFunc != nil {
		if err = chromedp.Run(ctx, fetch.Enable()); err != nil {
			log.Println(err)
			return nil, err
		}
		chromedp.ListenTarget(ctx, options.HookFunc(ctx))
	}

	// 创建 BrowserInstance
	id := bc.nextID
	bc.nextID++
	instance := NewBrowserInstance(id, browser, ctx, func() {
		cancel()
		cancelAlloc()
	})

	// 将浏览器实例添加到控制器中
	bc.instances[id] = instance

	return instance, nil
}

// CloseBrowser 关闭指定的浏览器实例
func (bc *BrowserController) CloseBrowser(id int) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	instance, exists := bc.instances[id]
	if !exists {
		return fmt.Errorf("browser instance with ID %d does not exist", id)
	}

	instance.Close()

	delete(bc.instances, id) // 从映射中移除
	return nil
}

// GetBrowserInstance 获取指定的浏览器实例
func (bc *BrowserController) GetBrowserInstance(id int) (*BrowserInstance, error) {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	instance, exists := bc.instances[id]
	if !exists {
		return nil, fmt.Errorf("browser instance with ID %d does not exist", id)
	}

	return instance, nil
}

// CloseAllBrowsers 关闭所有浏览器实例
func (bc *BrowserController) CloseAllBrowsers() {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	for id, instance := range bc.instances {
		instance.Close()
		delete(bc.instances, id)
	}
}

// GetBrowserCount 获取当前管理的浏览器实例数量
func (bc *BrowserController) GetBrowserCount() int {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	return len(bc.instances)
}
