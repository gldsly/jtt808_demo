/*
这里是jt808 数据编码和解码主要文件
如果想运行此demo, 请运行 server.go中的 StartGPSServer 函数
*/

package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"strconv"
	"strings"

	"github.com/imroc/biu"
)

// getSerialNumber 获取服务端流水号
func getSerialNumber() uint16 {
	lock.Lock()
	defer func() {
		lock.Unlock()
	}()
	o := serialNumber
	serialNumber++
	if serialNumber >= math.MaxUint16 {
		serialNumber = 0
	}
	return o
}

// convertData 转义数据
func convertData(flag int, data []byte) []byte {
	// flag = 1 转义数据中的 7E
	// flag = 0 还原数据中 7E
	// 转换规则:
	// 如果 0x7d(125) 后面是 0x02(2) 转义为 0x7e(126)
	// 如果 0x7d(125) 后面是 0x01(1) 转义为 0x7d(125)
	var result []byte
	var l = len(data)
	if flag == 1 {
		// 转义数据
		for i := 0; i < l; i++ {
			if data[i] == 126 {
				result = append(result, 125, 2)
			} else if data[i] == 125 {
				result = append(result, 125, 1)
			} else {
				result = append(result, data[i])
			}
		}
	} else {
		// 还原数据
		tag := false
		for i := 0; i < l; i++ {
			if tag {
				tag = false
				continue
			}
			if data[i] == 125 && data[i+1] == 2 {
				result = append(result, 126)
				tag = true
			} else if data[i] == 125 && data[i+1] == 1 {
				result = append(result, 125)
				tag = true
			} else {
				result = append(result, data[i])
			}
		}
	}
	return result

}

// generateMessageAttr 生成消息属性
func generateMessageAttr(attr messageHeadAttr) ([]byte, error) {
	bodyLength, err := dec2x(attr.BodyLength, 2)
	if err != nil {
		return nil, err
	}
	if len(bodyLength) < 10 {
		n := 10 - len(bodyLength)
		bodyLength = strJoin("", strings.Repeat("0", n), bodyLength)
	}
	binAttr := strJoin("", attr.Other, attr.SplitPack, attr.CryptoMethod, bodyLength)
	attrByte := biu.BinaryStringToBytes(binAttr)
	return attrByte, nil
}

// parseMsgHeaderAttr 解析消息头属性信息
func parseMsgHeaderAttr(binStr string) messageHeadAttr {
	var bodyLength = binStr[6:]

	var headAttr = messageHeadAttr{
		BodyLength:   binStr2DEC(bodyLength),
		CryptoMethod: binStr[3:6],
		SplitPack:    binStr[2:3],
		Other:        binStr[:2],
	}
	return headAttr
}

// parseMsgHeader 解析消息头
func parseMsgHeader(d []byte) (*messageHead, error) {
	var msgIDByte = d[:2]
	var msgAttrByte = d[2:4]
	var phoneByte = d[4:10]
	var serialNumByte = d[10:]

	// 解析消息头属性
	attr1 := biu.ToBinaryString(msgAttrByte[0])
	attr2 := biu.ToBinaryString(msgAttrByte[1])
	attrResult := parseMsgHeaderAttr(strings.Join([]string{attr1, attr2}, ""))

	serialNum, err := strconv.ParseInt(hex.EncodeToString(serialNumByte), 16, 64)
	if err != nil {
		return nil, errors.New("转换消息头消息流水号失败:" + err.Error())
	}

	var result = &messageHead{
		ID:        hex.EncodeToString(msgIDByte),
		Attr:      attrResult,
		Phone:     decodeBCD(phoneByte),
		SerialNum: uint16(serialNum),
		SPC:       nil,
	}

	return result, nil

}

// decodeMsg 解码消息
func decodeMsg(conn net.Conn) (*ProtocolData, error) {

	// 参考协议单字节使用标志位接收数据
	var allDataByte []byte
	var buff = make([]byte, 1)
	var readTag = false
	for {
		_, err := conn.Read(buff)
		if err != nil {
			if err == io.EOF {
				return nil, errEOF
			}
			return nil, err
		}
		if !readTag {
			// 寻找数据头
			if buff[0] == 126 {
				readTag = true
			} else {
				continue
			}
		} else {
			// 寻找数据尾
			if buff[0] == 126 {
				break
			}
			allDataByte = append(allDataByte, buff[0])
		}
	}

	// 转换数据(大端模式)
	var allData = make([]byte, len(allDataByte))
	err := binary.Read(bytes.NewBuffer(allDataByte[:]), binary.BigEndian, allData)
	if err != nil {
		return nil, errors.New("大端模式转换失败:" + err.Error())
	}

	// 全量还原转义数据
	requestData := convertData(DecodeMsg, allData)

	// 计算数据长度
	requestDataLength := len(requestData)

	// 验证校验码
	var checkSumNum = requestData[requestDataLength-1:][0]
	var s = requestData[0]
	var msgLength = requestDataLength - 1
	for i := 1; i < msgLength; i++ {
		s = s ^ requestData[i]
	}
	if s != checkSumNum {
		return nil, fmt.Errorf("消息校验失败->数据值:%v 计算值:%v", checkSumNum, s)
	}

	// 去除校验码
	requestData = requestData[:requestDataLength-1]
	// 解析头部数据
	msgHeaderInfo, err := parseMsgHeader(requestData[:12])
	if err != nil {
		return nil, errors.New("解析消息头数据失败:" + err.Error())
	}

	// 获取消息体数据
	var bodyData []byte
	if msgHeaderInfo.Attr.SplitPack == "1" {

		totalPack, err := strconv.ParseInt(hex.EncodeToString(requestData[12:14][:2]), 16, 64)
		if err != nil {
			return nil, errors.New("转换分包封装项分包总数失败:" + err.Error())
		}
		curNum, err := strconv.ParseInt(hex.EncodeToString(requestData[14:16]), 16, 64)
		if err != nil {
			return nil, errors.New("转换分包封装项分包总数失败:" + err.Error())
		}
		msgHeaderInfo.SPC = &SplitPackContent{
			Total:  uint16(totalPack),
			CurNum: uint16(curNum),
		}
		bodyData = requestData[16:]
	} else {
		bodyData = requestData[12:]
	}
	return &ProtocolData{
		Head: msgHeaderInfo,
		Body: bodyData,
	}, nil
}

// encodeMsg 编码消息
func encodeMsg(head *messageHead, body []byte) ([]byte, error) {

	tag, err := hex.DecodeString("7e")
	if err != nil {
		return nil, errors.New("编码标识符失败:" + err.Error())
	}
	messageID, err := hex.DecodeString(head.ID)
	if err != nil {
		return nil, errors.New("编码消息ID失败:" + err.Error())
	}
	messageAttr, err := generateMessageAttr(head.Attr)
	if err != nil {
		return nil, errors.New("编码消息属性失败:" + err.Error())
	}

	phone := encodeBCD(head.Phone)

	serialByte, err := dec2HexByte(int(head.SerialNum), 4)
	if err != nil {
		return nil, errors.New("转换消息头:流水号失败:" + err.Error())
	}

	pack := new(bytes.Buffer)
	// 写入消息头: 消息号
	err = binary.Write(pack, binary.BigEndian, messageID)
	// 写入消息头: 属性
	err = binary.Write(pack, binary.BigEndian, messageAttr)
	// 写入消息头: 手机号
	err = binary.Write(pack, binary.BigEndian, phone)
	// 写入消息头: 流水号
	err = binary.Write(pack, binary.BigEndian, serialByte)
	// 写入消息头: 封装项,如果有
	if head.SPC != nil {
		totalByte, err := dec2HexByte(int(head.SPC.Total), 4)
		if err != nil {
			return nil, errors.New("转换消息头:封装项分包总数失败:" + err.Error())
		}
		curNumByte, err := dec2HexByte(int(head.SPC.CurNum), 4)
		if err != nil {
			return nil, errors.New("转换消息头:封装项当前分包序号失败:" + err.Error())
		}

		err = binary.Write(pack, binary.BigEndian, totalByte)
		err = binary.Write(pack, binary.BigEndian, curNumByte)
	}
	// 写入消息体
	if body != nil {
		err = binary.Write(pack, binary.BigEndian, body)
	}

	// 计算校验码
	tmpPackBytes := pack.Bytes()
	tmpPackBytesLength := len(tmpPackBytes)
	checkSum := tmpPackBytes[0]
	for i := 1; i < tmpPackBytesLength; i++ {
		checkSum = checkSum ^ tmpPackBytes[i]
	}

	// 写入校验码
	err = binary.Write(pack, binary.BigEndian, checkSum)

	// 末尾检查err是否有值
	if err != nil {
		return nil, errors.New("编码数据失败:" + err.Error())
	}

	// 转义数据,除标志位外
	finalData := convertData(EncodeMsg, pack.Bytes())

	// 加标志位
	var fullData []byte
	fullData = append(fullData, tag...)
	fullData = append(fullData, finalData...)
	fullData = append(fullData, tag...)

	return fullData, nil
}

// SendMsgToTerm 发送数据到终端
// @param c chan int "传入无缓冲chan int,发送数据完成会向chan写入值,用于超时控制"
// @param conn net.Conn "终端连接句柄"
// @param msgID string "消息号ID(16进制字符串,2字节)"
// @param body []byte "消息体内容"
func SendMsgToTerm(c chan int, conn net.Conn, msgID string, body []byte) {
	defer func() {
		c <- 1
	}()

	var bodyLength int
	if body != nil {
		bodyLength = len(body)
	} else {
		bodyLength = 0
	}

	// 构建消息回复终端
	newMsg := &messageHead{
		ID: msgID,
		Attr: messageHeadAttr{
			BodyLength:   bodyLength,
			CryptoMethod: "000",
			SplitPack:    "0",
			Other:        "00",
		},
		Phone:     "017562610880",
		SerialNum: getSerialNumber(),
		SPC:       nil,
	}

	d, err := encodeMsg(newMsg, body)
	if err != nil {
		log.Printf("[发送数据]连接地址:%s 数据编码构建失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}

	_, err = conn.Write(d)
	if err != nil {
		log.Printf("[发送数据]连接地址:%s 数据发送失败:%s", conn.RemoteAddr().String(), err.Error())
		// 删除注册连接
		return
	}
	return
}
