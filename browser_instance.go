package browsers

import (
	"context"
	"fmt"
	"github.com/chromedp/cdproto/network"
	"github.com/chromedp/cdproto/runtime"
	"log"
	"net/http"
	"sync"

	"github.com/chromedp/chromedp"
)

// BrowserInstance 表示一个浏览器实例
type BrowserInstance struct {
	ID      int                // 浏览器实例的唯一标识
	Browser *chromedp.Context  // 浏览器实例
	Ctx     context.Context    // 上下文
	Cancel  context.CancelFunc // 取消函数
	Closed  bool               // 标记浏览器是否已关闭
	mu      sync.Mutex         // 用于保护 Closed 状态的互斥锁
}

// NewBrowserInstance 创建一个新的浏览器实例
func NewBrowserInstance(id int, browser *chromedp.Context, ctx context.Context, cancel context.CancelFunc) *BrowserInstance {
	return &BrowserInstance{
		ID:      id,
		Browser: browser,
		Ctx:     ctx,
		Cancel:  cancel,
		Closed:  false,
	}
}

// Close 关闭浏览器实例
func (bi *BrowserInstance) Close() error {
	bi.mu.Lock()
	defer bi.mu.Unlock()

	if bi.Closed {
		// 如果已经关闭，直接返回
		return nil
	}

	// 1. 确保取消所有挂起的浏览器任务
	if err := chromedp.Cancel(bi.Ctx); err != nil {
		log.Printf("Failed to cancel chromedp context for browser instance %d: %v", bi.ID, err)
	}

	// 2. 释放上下文并关闭浏览器
	if bi.Cancel != nil {
		bi.Cancel() // 取消浏览器上下文
	}

	// 3. 标记浏览器已关闭
	bi.Closed = true

	// 4. 记录日志 (可选)
	log.Printf("Browser instance %d has been closed", bi.ID)

	return nil
}

// IsClosed 检查浏览器实例是否已关闭
func (bi *BrowserInstance) IsClosed() bool {
	bi.mu.Lock()
	defer bi.mu.Unlock()
	return bi.Closed
}

func (bi *BrowserInstance) Goto(url string, beforeNavigate ...func(ctx context.Context) error) error {
	// 如果浏览器已关闭，直接返回错误
	if bi.IsClosed() {
		return fmt.Errorf("浏览器已关闭")
	}
	// 执行导航操作
	return chromedp.Run(bi.Ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			for _, cb := range beforeNavigate {
				err := cb(ctx)
				if err != nil {
					return err
				}
			}
			return nil
		}),
		chromedp.Navigate(url),
	)
}

func (bi *BrowserInstance) GetCookies() ([]*http.Cookie, error) {
	// 检查浏览器是否已关闭
	if bi.IsClosed() {
		return nil, fmt.Errorf("浏览器已关闭")
	}

	// 创建一个容器来接收 cookies
	var cookies []*http.Cookie

	// 获取 cookies
	err := chromedp.Run(bi.Ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			cks, err := network.GetCookies().Do(ctx)
			if err != nil {
				return err
			}
			cookies = convertCookies(cks)
			return nil
		}),
	)
	if err != nil {
		return nil, fmt.Errorf("获取 cookies 失败: %v", err)
	}

	// 返回获取到的 cookies
	return cookies, nil
}

func (bi *BrowserInstance) SabaFetch(eval string) *BrowserResponse {
	var data = make(map[string]*BrowserResponse)
	err := chromedp.Run(bi.Ctx,
		chromedp.ActionFunc(func(ctx context.Context) error {
			return chromedp.Evaluate(fmt.Sprintf(`(async function() {var c = %v;return {"dst":c};})()`, eval),
				&data,
				func(p *runtime.EvaluateParams) *runtime.EvaluateParams {
					return p.WithAwaitPromise(true)
				},
			).Do(ctx)
		}),
	)

	if val, ok := data["dst"]; ok {
		return val
	}
	b := &BrowserResponse{
		Data:  "",
		Error: "nil Response",
		Token: "",
	}
	if err != nil {
		b.Error = err.Error()
	}
	return b
}

func convertCookies(netCookies []*network.Cookie) []*http.Cookie {
	httpCookies := []*http.Cookie{}

	for _, netCookie := range netCookies {
		httpCookie := &http.Cookie{
			Name:     netCookie.Name,
			Value:    netCookie.Value,
			Path:     netCookie.Path,
			Domain:   netCookie.Domain,
			Secure:   netCookie.Secure,
			HttpOnly: netCookie.HTTPOnly,
		}

		httpCookies = append(httpCookies, httpCookie)
	}

	return httpCookies
}

type BrowserResponse struct {
	Data  string `json:"data,omitempty"`
	Error string `json:"error,omitempty"`
	Token string `json:"token,omitempty"`
}
