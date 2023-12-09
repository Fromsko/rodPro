# 基于 Rod 轻度魔改的 Pro

## 下载

```shell
go get github.com/Fromsko/rodPro@latest
```

## 使用

> 支持 本地化 | Docker Chorme | Websocket

### 配套项目

[Go-Rod 实现的课表项目](https://github.com/Fromsko/GoSchedule)

### 简单封装

```go
package main

import (
	"fmt"
	"io/fs"
	"os"

	rod "github.com/Fromsko/rodPro"
	"github.com/Fromsko/rodPro/lib/launcher"
)

type WebObject struct {
	BaseUrl string       // 基础地址
	Page    *rod.Page    // 页面对象
	Browser *rod.Browser // 浏览器对象
}

// InitWeb 初始化浏览器
func InitWeb(url, ws string) (web *WebObject) {
	//启动浏览器
	var browser *rod.Browser

	// 创建浏览器实例
	if ws != "" {
		browser = rod.New().ControlURL(ws).MustConnect()
	} else {
		u := launcher.New().MustLaunch()
		browser = rod.New().ControlURL(u).MustConnect()
	}

	// 新建一个页面
	page := browser.MustPage()

	return &WebObject{
		BaseUrl: url,
		Page:    page,
		Browser: browser,
	}
}

// SearchParams 查找参数
func (w *WebObject) SearchParams(text string) (*rod.SearchResult, bool) {
	search, err := w.Page.Search(text)
	if err != nil {
		return search, false
	}
	return search, search.ResultCount != 0
}

// Screenshot 通用截屏
func Screenshot(obj any, filename string) {
	type Screenshoter interface {
		MustScreenshot(toFile ...string) []byte
	}

	var p Screenshoter

	switch obj := obj.(type) {

	case *rod.Page:
		p = obj.MustWaitStable()
	case *rod.Element:
		p = obj
	}

	p.MustScreenshot(filename)
}

// go run . -rod=show,devtools
func main() {
	web := InitWeb("https://github.com", "")

	web.Page.MustNavigate(web.BaseUrl).MustWaitLoad()

	if elements, ok := web.SearchParams("Sign up"); !ok {
		fmt.Println("没找到")
	} else {
		Screenshot(elements.First, "Github-Sign.png")
	}

	html, _ := web.Page.HTML()

	_ = os.WriteFile(
		"Github.html",
		[]byte(html),
		fs.FileMode(os.O_WRONLY),
	)

	defer func() {
		Screenshot(web.Page, "Github.png")
		web.Browser.MustClose()
	}()
}
```

## 魔改部分(优化)

> 官方的 `query.go -> search` 函数在多次获取页面时, [有概率]出现死锁导致内存溢出
>
> 针对 重试机制进行了优化, 默认设置为 3 次重试。

```go
// RetryOptions 定义了重试机制的配置。
type RetryOptions struct {
    Context    context.Context             // 用于控制重试过程的上下文。
    Sleeper    func(context.Context) error // 在重试之间等待的 Sleeper 函数。
    MaxRetries int                         // 最大重试次数。
}

// NewRetry 基于提供的 RetryOptions 实现了一个重试机制。
// 函数 `fn` 将执行最多 MaxRetries 次，直到它指示停止或发生错误。
func NewRetry(options RetryOptions, fn func() (stop bool, err error)) error {
    for i := 0; i < options.MaxRetries; i++ {
        stop, err := fn()
        if stop {
            return err
        }
        // 使用 options 中的 Sleeper 函数在下一次重试之前等待。
        err = options.Sleeper(options.Context)
        if err != nil {
            return err
        }
    }
    return nil // 如果达到最大重试次数而未成功，则返回 nil。
}

func (p *Page) Search(query string) (*SearchResult, error) {
	sr := &SearchResult{
		page:    p,
		restore: p.EnableDomain(proto.DOMEnable{}),
	}

	// TODO: 引入重试机制 | 设置默认 3 轮
	retryOptions := RetryOptions{
		Context:    p.ctx,
		Sleeper:    p.sleeper(),
		MaxRetries: 3,
	}

	// Use the NewRetry function with the defined options and search logic.
	err := NewRetry(retryOptions, func() (bool, error) {
            if sr.DOMPerformSearchResult != nil {
                // Discard previous search results before performing a new search.
                _ = proto.DOMDiscardSearchResults{SearchID: sr.SearchID}.Call(p)
            }
            // NOTE: 原有代码
        }
    )
    // NOTE: 原有代码
    return sr, nil
}
```

#### 鸣谢

> Rod 官方项目

# Overview

[![Go Reference](https://pkg.go.dev/badge/github.com/Fromsko/rodPro.svg)](https://pkg.go.dev/github.com/Fromsko/rodPro)
[![Discord Chat](https://img.shields.io/discord/719933559456006165.svg)][discord room]

## [Documentation](https://go-rod.github.io/) | [API reference](https://pkg.go.dev/github.com/Fromsko/rodPro?tab=doc) | [FAQ](https://go-rod.github.io/#/faq/README)

Rod is a high-level driver directly based on [DevTools Protocol](https://chromedevtools.github.io/devtools-protocol).
It's designed for web automation and scraping for both high-level and low-level use, senior developers can use the low-level packages and functions to easily
customize or build up their own version of Rod, the high-level functions are just examples to build a default version of Rod.

[中文 API 文档](https://pkg.go.dev/github.com/go-rod/go-rod-chinese)

## Features

- Chained context design, intuitive to timeout or cancel the long-running task
- Auto-wait elements to be ready
- Debugging friendly, auto input tracing, remote monitoring headless browser
- Thread-safe for all operations
- Automatically find or download [browser](lib/launcher)
- High-level helpers like WaitStable, WaitRequestIdle, HijackRequests, WaitDownload, etc
- Two-step WaitEvent design, never miss an event ([how it works](https://github.com/ysmood/goob))
- Correctly handles nested iframes or shadow DOMs
- No zombie browser process after the crash ([how it works](https://github.com/ysmood/leakless))
- [CI](https://github.com/Fromsko/rodPro/actions) enforced 100% test coverage

## Examples

Please check the [examples_test.go](examples_test.go) file first, then check the [examples](lib/examples) folder.

For more detailed examples, please search the unit tests.
Such as the usage of method `HandleAuth`, you can search all the `*_test.go` files that contain `HandleAuth`,
for example, use Github online [search in repository](https://github.com/Fromsko/rodPro/search?q=HandleAuth&unscoped_q=HandleAuth).
You can also search the GitHub [issues](https://github.com/Fromsko/rodPro/issues) or [discussions](https://github.com/Fromsko/rodPro/discussions),
a lot of usage examples are recorded there.

[Here](lib/examples/compare-chromedp) is a comparison of the examples between rod and Chromedp.

If you have questions, please raise an [issues](https://github.com/Fromsko/rodPro/issues)/[discussions](https://github.com/Fromsko/rodPro/discussions) or join the [chat room][discord room].

## Join us

Your help is more than welcome! Even just open an issue to ask a question may greatly help others.

Please read [How To Ask Questions The Smart Way](http://www.catb.org/~esr/faqs/smart-questions.html) before you ask questions.

We use Github Projects to manage tasks, you can see the priority and progress of the issues [here](https://github.com/Fromsko/rodPro/projects).

If you want to contribute please read the [Contributor Guide](.github/CONTRIBUTING.md).

[discord room]: https://discord.gg/CpevuvY
