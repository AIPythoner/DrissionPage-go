package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"os/signal"

	dp "github.com/AIPythoner/DrissionPage-go"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	browser := flag.Bool("browser", false, "使用 Chromium")
	bin := flag.String("bin", "", "Chromium 可执行文件路径")
	address := flag.String("address", "", "接管的 CDP 地址")
	target := flag.String("url", "", "目标 URL")
	locator := flag.String("locator", "", "元素定位器；留空输出 HTML")
	headless := flag.Bool("headless", true, "无头模式")
	version := flag.Bool("version", false, "显示对应的 Python 源码版本")
	flag.Parse()
	if *version {
		fmt.Println("DrissionPage Go port; Python source", dp.SourceVersion)
		return nil
	}
	if *target == "" {
		return fmt.Errorf("请提供 -url")
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt)
	defer stop()
	if *browser {
		o := dp.NewChromiumOptions().SetBrowserPath(*bin).SetAddress(*address).SetHeadless(*headless)
		b, e := dp.NewChromium(ctx, o)
		if e != nil {
			return e
		}
		defer b.Close()
		tab, e := b.NewTab(ctx, *target)
		if e != nil {
			return e
		}
		if *locator != "" {
			els, e := tab.Eles(ctx, *locator)
			if e != nil {
				return e
			}
			for _, el := range els {
				text, e := el.Text(ctx)
				if e != nil {
					return e
				}
				fmt.Println(text)
			}
		} else {
			h, e := tab.HTML(ctx)
			if e != nil {
				return e
			}
			fmt.Println(h)
		}
		return nil
	}
	p, e := dp.NewSessionPage()
	if e != nil {
		return e
	}
	defer p.Close()
	if _, e = p.Get(ctx, *target); e != nil {
		return e
	}
	if *locator != "" {
		els, e := p.Eles(*locator)
		if e != nil {
			return e
		}
		for _, el := range els {
			fmt.Println(el.Text())
		}
	} else {
		h, e := p.HTML()
		if e != nil {
			return e
		}
		fmt.Println(h)
	}
	return nil
}
