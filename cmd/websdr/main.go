package main

import (
	"context"
	"flag"
	"fmt"
	"io/fs"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/iamoumeng/websdr/internal/receiver"
	"github.com/iamoumeng/websdr/internal/server"
	webembed "github.com/iamoumeng/websdr/web"
)

func main() {
	host := flag.String("host", "0.0.0.0", "HTTP 监听地址")
	port := flag.Int("port", 8080, "HTTP 服务端口")
	freq := flag.Uint64("freq", 100000000, "初始中心频率 (Hz)")
	gain := flag.Float64("gain", 30, "增益 (dB)")
	agc := flag.Bool("agc", false, "启用自动增益")
	direct := flag.Int("direct", 0, "已弃用：HF 现按调谐频率自动切换（<24 MHz 开 Q 通道）")
	device := flag.Uint("device", 0, "RTL-SDR 设备索引")
	flag.Parse()

	count := receiver.DeviceCount()
	if count == 0 {
		log.Fatal("未检测到 RTL-SDR 设备，请确认设备已插入")
	}
	log.Printf("检测到 %d 个 RTL-SDR 设备", count)

	cfg := receiver.DefaultConfig()
	cfg.DeviceIndex = *device
	cfg.CenterFreq = *freq
	cfg.Gain = float32(*gain)
	cfg.AGC = *agc
	_ = direct // legacy flag, auto mode ignores manual HF setting

	rx, err := receiver.New(cfg)
	if err != nil {
		log.Fatalf("初始化接收器失败: %v", err)
	}
	defer rx.Close()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := rx.Start(ctx); err != nil {
		log.Fatalf("启动接收失败: %v", err)
	}

	webContent, _ := fs.Sub(webembed.FS, "dist")

	hub := server.NewHub(rx)
	go hub.RunBroadcast(ctx)

	mux := http.NewServeMux()
	mux.Handle("/", http.FileServer(http.FS(webContent)))
	mux.HandleFunc("/ws", hub.HandleWS)
	mux.HandleFunc("/api/meteor/catalog", server.HandleMeteorCatalog)
	mux.HandleFunc("/api/meteor/tle", server.HandleMeteorTLE)
	mux.HandleFunc("/api/satellite/catalog", server.HandleSatelliteCatalog)
	mux.HandleFunc("/api/satellite/tle", server.HandleSatelliteTLE)

	addr := fmt.Sprintf("%s:%d", *host, *port)
	log.Printf("WebSDR 已启动: http://127.0.0.1:%d  (监听 %s)", *port, addr)
	log.Printf("中心频率: %.3f MHz, 采样率: %d Hz", float64(cfg.CenterFreq)/1e6, cfg.SampleRate)

	srv := &http.Server{Addr: addr, Handler: mux}
	go func() {
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP 服务错误: %v", err)
		}
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig
	log.Println("正在关闭...")
	cancel()
	srv.Shutdown(context.Background())
}
