package main

import (
	"errors"
	"sync"
)

var (
	errEOF              = errors.New("eof")
	serialNumber uint16 = 0
	lock         sync.Mutex
)

// 转义数据标志位
const (
	DecodeMsg = iota
	EncodeMsg
)

// SplitPackContent 数据分包时:消息头带的分包信息
type SplitPackContent struct {
	Total  uint16
	CurNum uint16 `comment:"从1开始计数"`
}

// messageAttr 消息头属性
type messageHeadAttr struct {
	BodyLength   int
	CryptoMethod string
	SplitPack    string
	Other        string
}

// messageHead 消息头
type messageHead struct {
	ID        string
	Attr      messageHeadAttr
	Phone     string
	SerialNum uint16
	SPC       *SplitPackContent
}

// ProtocolData jt/t808-2013 协议规定包数据
type ProtocolData struct {
	Head *messageHead
	Body []byte
}
