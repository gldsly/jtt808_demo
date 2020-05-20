/*
socket长连接管理相关方法和函数
*/

package main

import (
	"log"
	"net"
	"sync"
	"time"
)

var (
	// OnlineClient 客户端网络地址和连接句柄映射
	OnlineClient = make(map[string]*TerminalInfo)
	// ClientAddrToken 客户端令牌和网络地址映射
	ClientAddrToken = make(map[string]string)
	connTableLock   sync.Mutex
)

// TerminalInfo 终端连接信息
type TerminalInfo struct {
	Addr      string
	TimeStamp int64
	Conn      net.Conn
	CarNumber string
}

// addClient 添加已连接设备
func addClient(token string, t *TerminalInfo) {
	connTableLock.Lock()
	defer func() {
		log.Println("新的客户端已添加:", token)
		connTableLock.Unlock()
	}()

	OnlineClient[token] = t
	ClientAddrToken[t.Addr] = token
}

// removeClient 移除已连接客户端
func removeClient(token string) {
	connTableLock.Lock()
	defer func() {
		connTableLock.Unlock()
	}()
	term, ok := OnlineClient[token]

	if !ok || term == nil {
		return
	}
	conn := term.Conn
	log.Printf("[GPS Server] 连接: %s 车牌号: %s 连接断开...", conn.RemoteAddr().String(), term.CarNumber)
	delete(ClientAddrToken, conn.RemoteAddr().String())
	_ = conn.Close()
	delete(OnlineClient, token)
}

// updateClientState 更新客户端的时间
func updateClientState(token string) {
	connTableLock.Lock()
	defer func() {
		connTableLock.Unlock()
	}()
	OnlineClient[token].TimeStamp = time.Now().Unix()
}

// removeTimeoutClient 移除超时的客户端
func removeTimeoutClient() {

	log.Println("[GPS Server]心跳 监控 进程启动...")
	var count = 0
	for range time.Tick(time.Duration(1) * time.Second) {
		nowTime := time.Now().Unix()
		if count >= 60 {
			count = 0
		}

		// 检测已连接客户端是否超时
		if count%30 == 0 {
			// log.Println("[GPS Server] 开始检测客户端连接状态...")
			for token, val := range OnlineClient {
				if int(nowTime-val.TimeStamp) > 70 {
					// log.Println("关闭的客户端:", token, int(nowTime-val.TimeStamp))
					removeClient(token)
				}
			}
		}

		// 删除超时车辆缓存
		if count%60 == 0 {
			// log.Println("[GPS Server] 开始清除车辆缓存超时数据...")
			for key, cache := range cacheVehicleLocationData {
				if int(nowTime-cache.TimeStamp) > 86400 {
					// log.Println("删除的车辆:", key, nowTime, cache.TimeStamp, 86400)
					delete(cacheVehicleLocationData, key)
				}
			}
		}
		count++
	}

}
