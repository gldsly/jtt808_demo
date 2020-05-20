/*
socket服务端启动
*/

package main

import (
	"log"
	"net"
)

// gpsConnHandler gps服务请求处理
func gpsConnHandler(conn net.Conn) {
	defer func() {
		log.Println("连接:", conn.RemoteAddr().String(), "断开...")
		token := ClientAddrToken[conn.RemoteAddr().String()]
		removeClient(token)
	}()
	log.Printf("GPS设备终端已连接:%s\n", conn.RemoteAddr().String())
	for {
		requestData, err := decodeMsg(conn)
		if err != nil {
			if err == errEOF {
				return
			}
			if _, ok := err.(net.Error); ok {
				return
			}
			log.Printf("解码消息错误: %s\n", err.Error())
			continue
		}
		if f, ok := messageMap[requestData.Head.ID]; ok {
			f(requestData, conn)
		} else {
			log.Println("暂不支持的终端消息,消息ID:", requestData.Head.ID)
		}
	}
}

// StartGPSServer 启动GPS服务器
func StartGPSServer() {
	var err error
	go removeTimeoutClient()
	log.Println("[GPS Server] GPS服务端启动...")
	listener, err := net.Listen("tcp", ":8901")
	if err != nil {
		log.Fatalf("[GPS Server] 启动GPS服务器监听失败:%s", err.Error())
	}

	for {
		conn, err := listener.Accept()
		if err != nil {
			log.Printf("[GPS Server] 新的终端连接失败:%s", err.Error())
			continue
		}
		if err != nil {
			log.Printf("[GPS Server] 设置客户端读超时失败:%s", err.Error())
		}
		go gpsConnHandler(conn)
	}
}
