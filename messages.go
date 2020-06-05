/*
jt/t 808 消息体处理函数方法
*/

package main

import (
	"encoding/hex"
	"fmt"
	"log"
	"net"
	"strconv"

	"bytes"
	"context"
	"encoding/binary"
	"time"

	"golang.org/x/text/encoding/simplifiedchinese"
)

var (
	// messageMap 支持的终端消息映射
	messageMap = map[string]func(*ProtocolData, net.Conn){
		"0100": TerminalRegistration,
		"0001": TerminalUniversalReply,
		"0002": TerminalKeepAlive,
		"0102": TerminalAuth,
		"0200": UploadLocation,
		"0201": QueryLocationReply,
		"0704": MultiUploadLocation,
	}
)

// additionalDataParse 位置信息汇报附加信息读取
func additionalDataParse(d []byte, result map[string]int) {

	additionalMsgID := hex.EncodeToString(d[:1])
	if len(additionalMsgID) != 2 {
		additionalMsgID = strJoin("", "0", additionalMsgID)
	}
	mapKey := locationAdditionalMsgIDMap[additionalMsgID]

	msgLength, err := strconv.ParseInt(hex.EncodeToString(d[1:2]), 16, 64)
	if err != nil {
		log.Printf("[位置信息汇报(附加信息)]解析:%s 失败:%s", mapKey, err.Error())
		return
	}

	if additionalMsgID == "01" || additionalMsgID == "03" || additionalMsgID == "30" || additionalMsgID == "31" {
		value, err := strconv.ParseInt(hex.EncodeToString(d[2:msgLength+2]), 16, 64)
		if err != nil {
			log.Printf("[位置信息汇报(附加信息)]解析:%s 的数据值失败:%s", mapKey, err.Error())
			return
		}
		result[mapKey] = int(value)
	} else {
		log.Printf("[位置信息汇报(附加信息)]不支持:%s 解析", mapKey)
	}

	// 判断是否存在
	if len(d)-int(2+msgLength) > 0 {
		additionalDataParse(d[2+msgLength:], result)
	}
	return
}

// TerminalUniversalReply 终端通用应答
func TerminalUniversalReply(request *ProtocolData, conn net.Conn) {
	serialNum, err := strconv.ParseInt(hex.EncodeToString(request.Body[:2]), 16, 64)
	if err != nil {
		log.Printf("[终端通用应答]连接地址:%s 解析消息流水号失败:%s", conn.RemoteAddr().String(), err.Error())
	}
	msgID := hex.EncodeToString(request.Body[2:4])
	result := hex.EncodeToString(request.Body[4:])

	fmt.Println("[终端通用应答]", serialNum, msgID, result)
}

// ServerUniversalReply 平台通用应答
func ServerUniversalReply(conn net.Conn, body *DefServerUniversalReply) {
	// 简单数据校验
	if len(body.Result) != 2 || len(body.ReplyID) != 4 {
		log.Printf("[平台通用应答]连接地址:%s 数据长度校验失败", conn.RemoteAddr().String())
		return
	}
	// 编码数据
	terminalSerialNumber, err := dec2HexByte(int(body.ReplySerialNum), 4)
	if err != nil {
		log.Printf("[平台通用应答]连接地址:%s 编码应答流水号失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}
	result, err := hex.DecodeString(body.Result)
	if err != nil {
		log.Printf("[平台通用应答]连接地址:%s 编码应答结果失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}
	replyID, err := hex.DecodeString(body.ReplyID)
	if err != nil {
		log.Printf("[平台通用应答]连接地址:%s 编码应答消息ID失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}

	// 构建数据包
	bodyPack := new(bytes.Buffer)

	err = binary.Write(bodyPack, binary.BigEndian, terminalSerialNumber)
	err = binary.Write(bodyPack, binary.BigEndian, replyID)
	err = binary.Write(bodyPack, binary.BigEndian, result)

	if err != nil {
		log.Printf("[平台通用应答]连接地址:%s 应答消息包构建失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}

	bodyByte := bodyPack.Bytes()
	signal := make(chan int)
	ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(10))
	go SendMsgToTerm(signal, conn, "8001", bodyByte)
	for {
		select {
		case <-ctx.Done():
			log.Printf("[平台通用应答]连接地址:%s 应答消息发送失败,超时", conn.RemoteAddr().String())
			cancelFunc()
			return
		case <-signal:
			cancelFunc()
			return
		}
	}
}

// TerminalKeepAlive 终端心跳
func TerminalKeepAlive(request *ProtocolData, conn net.Conn) {

	// log.Println("[终端心跳] 刷新客户端状态:", conn.RemoteAddr().String())

	// 刷新客户端状态
	token := ClientAddrToken[conn.RemoteAddr().String()]
	updateClientState(token)

	// 平台通用回复
	body := &DefServerUniversalReply{
		ReplySerialNum: request.Head.SerialNum,
		ReplyID:        "0002",
		Result:         ServerUniversalReplyStatus["成功"],
	}
	ServerUniversalReply(conn, body)
	return
}

// TerminalRegistration 终端注册
func TerminalRegistration(request *ProtocolData, conn net.Conn) {
	var err error

	// 解析数据
	provinceID, err := strconv.ParseInt(hex.EncodeToString(request.Body[:2]), 16, 64)
	cityID, err := strconv.ParseInt(hex.EncodeToString(request.Body[2:4]), 16, 64)
	if err != nil {
		log.Printf("[终端注册]连接地址:%s 解析省/市域ID错误:%s", conn.RemoteAddr().String(), err.Error())
		return
	}

	manufacturerID := string(request.Body[4:9])

	terminalType := string(request.Body[9:29])
	terminalID := string(request.Body[29:36])
	licensePlateColor, err := strconv.ParseInt(hex.EncodeToString(request.Body[36:37]), 16, 64)
	if err != nil {
		log.Printf("[终端注册]连接地址:%s 解析车牌颜色错误:%s", conn.RemoteAddr().String(), err.Error())
		return
	}

	gbkDecoder := simplifiedchinese.GBK.NewDecoder()
	vehicleIdentification, err := gbkDecoder.Bytes(request.Body[37:])
	if err != nil {
		log.Printf("[终端注册]连接地址:%s 使用GBK解析车辆标识失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}

	info := DefTerminalRegistration{
		ProvinceID:            int(provinceID),
		CityID:                int(cityID),
		ManufacturerID:        manufacturerID,
		TerminalType:          terminalType,
		TerminalID:            terminalID,
		LicensePlateColor:     LicensePlateColorMap[int(licensePlateColor)],
		VehicleIdentification: string(vehicleIdentification),
	}

	var handleState = "成功"
	var authCode string

	// 校验车辆: 校验失败 handleState = "失败"
	fmt.Println(info)

	// 应答终端结果: 成功注册返回鉴权码
	if handleState == "成功" {
		// 更新数据库信息

	}

	body := &DefTerminalRegistrationReply{
		ReplySerialNum: request.Head.SerialNum,
		Result:         RegistrationReplyStatus[handleState],
		AuthCode:       authCode,
	}
	RegistrationReply(conn, body)
	return
}

// RegistrationReply 终端注册应答
func RegistrationReply(conn net.Conn, body *DefTerminalRegistrationReply) {
	// 构建消息体数据包
	log.Println("发送终端注册应答:", conn.RemoteAddr().String())
	bodyPack := new(bytes.Buffer)
	terminalSerialNumber, err := dec2HexByte(int(body.ReplySerialNum), 4)
	if err != nil {
		log.Printf("[终端注册应答]连接地址:%s 编码应答流水号失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}
	err = binary.Write(bodyPack, binary.BigEndian, terminalSerialNumber)

	result, err := hex.DecodeString(body.Result)
	if err != nil {
		log.Printf("[终端注册应答]连接地址:%s 编码应答结果失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}
	err = binary.Write(bodyPack, binary.BigEndian, result)
	if body.AuthCode != "" {
		authCode, err := hex.DecodeString(body.AuthCode)
		if err != nil {
			log.Printf("[终端注册应答]连接地址:%s 编码鉴权码失败:%s", conn.RemoteAddr().String(), err.Error())
			return
		}
		err = binary.Write(bodyPack, binary.BigEndian, authCode)
	}
	if err != nil {
		log.Printf("[终端注册应答]连接地址:%s 注册应答消息包构建失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}

	bodyByte := bodyPack.Bytes()

	// 构建消息回复终端
	newMsg := &messageHead{
		ID: "8100",
		Attr: messageHeadAttr{
			BodyLength:   len(bodyByte),
			CryptoMethod: "000",
			SplitPack:    "0",
			Other:        "00",
		},
		Phone:     "017562610880",
		SerialNum: getSerialNumber(),
		SPC:       nil,
	}

	d, err := encodeMsg(newMsg, bodyByte)
	if err != nil {
		log.Printf("[终端注册应答]连接地址:%s 回复消息构建失败:%s", conn.RemoteAddr().String(), err.Error())
		return
	}
	_, err = conn.Write(d)
	if err != nil {
		log.Printf("[终端注册应答]连接地址:%s 回复消息发送失败:%s", conn.RemoteAddr().String(), err.Error())
		// 删除注册连接
		return
	}
	return
}

// TerminalAuth 终端鉴权
func TerminalAuth(request *ProtocolData, conn net.Conn) {
	log.Println("终端鉴权消息:", conn.RemoteAddr().String())
	var handleState = "成功"
	token := hex.EncodeToString(request.Body)
	// 验证鉴权码: 失败时 -> handleState = "失败"

	if handleState == "成功" {
		if _, ok := OnlineClient[token]; ok {
			removeClient(token)
		}
		newClient := &TerminalInfo{
			Addr:      conn.RemoteAddr().String(),
			TimeStamp: time.Now().Unix(),
			Conn:      conn,
			CarNumber: "", // 鉴权成功时查询到的车辆标识数据
		}
		addClient(token, newClient)
	}

	body := &DefServerUniversalReply{
		ReplySerialNum: request.Head.SerialNum,
		ReplyID:        "0102",
		Result:         ServerUniversalReplyStatus[handleState],
	}
	ServerUniversalReply(conn, body)
	return
}

// UploadLocation 位置信息汇报
func UploadLocation(request *ProtocolData, conn net.Conn) {
	// log.Println("[位置信息汇报] 客户端:", conn.RemoteAddr().String())
	var handleState string

	// 报警标志
	// alarmTag := biu.BytesToBinaryString(request.Body[:4])
	// 状态
	// state := biu.BytesToBinaryString(request.Body[4:8])
	// 纬度
	latitude, err := strconv.ParseInt(hex.EncodeToString(request.Body[8:12]), 16, 64)
	if err != nil {
		log.Printf("[位置信息汇报]连接地址:%s 解析纬度失败:%s", conn.RemoteAddr().String(), err.Error())
		handleState = "失败"
		return
	}
	// 经度
	longitude, err := strconv.ParseInt(hex.EncodeToString(request.Body[12:16]), 16, 64)
	if err != nil {
		log.Printf("[位置信息汇报]连接地址:%s 解析经度失败:%s", conn.RemoteAddr().String(), err.Error())
		handleState = "失败"
		return
	}
	// 高程
	// elevation, err := strconv.ParseInt(hex.EncodeToString(request.Body[16:18]), 16, 64)
	// if err != nil {
	// 	log.Printf("[位置信息汇报]连接地址:%s 解析高程失败:%s", conn.RemoteAddr().String(), err.Error())
	// 	handleState = "失败"
	// 	return
	// }
	// 速度
	speed, err := strconv.ParseInt(hex.EncodeToString(request.Body[18:20]), 16, 64)
	if err != nil {
		log.Printf("[位置信息汇报]连接地址:%s 解析速度失败:%s", conn.RemoteAddr().String(), err.Error())
		handleState = "失败"
		return
	}
	// 方向(方位角)
	direction, err := strconv.ParseInt(hex.EncodeToString(request.Body[20:22]), 16, 64)
	if err != nil {
		log.Printf("[位置信息汇报]连接地址:%s 解析方位角失败:%s", conn.RemoteAddr().String(), err.Error())
		handleState = "失败"
		return
	}
	uploadTime := decodeBCD(request.Body[22:28])

	handleState = "成功"
	// 位置基本信息解析完成
	// log.Printf("暂不处理的数据: 报警标志:%s 状态:%s 高程:%d 协议速度:%d\n", alarmTag, state, elevation, speed)

	longStr := strconv.FormatFloat(float64(longitude)/1000000, 'f', 6, 64)
	latStr := strconv.FormatFloat(float64(latitude)/1000000, 'f', 6, 64)

	// 判断是否携带附加消息
	// 位置基本信息只有 28 字节,如果消息头描述长度大于此值,则有附加信息
	additionalInfoMap := make(map[string]int)
	if request.Head.Attr.BodyLength > 28 {
		additionalDataParse(request.Body[28:], additionalInfoMap)
	}

	token := ClientAddrToken[conn.RemoteAddr().String()]
	termInfo := OnlineClient[token]
	// 查询缓冲 ? 命中 如果上次数据速度不为0则保存 : 未命中 直接保存
	v, ok := cacheVehicleLocationData[termInfo.CarNumber]
	currentDataSpeed := speed
	var saveTag = false
	if ok {
		// 本次速度不为 0 并且速度 大于 5km/h 进行数据存储
		if currentDataSpeed != 0 && currentDataSpeed > 51 {
			saveTag = true
			// 上次速度不为0, 本次速度为 0   保存
		} else if v.Speed != 0 && currentDataSpeed == 0 {
			if v.PerState {
				saveTag = true
			}
		}
	} else {
		// 车辆数据未缓存, 保存
		saveTag = true
		cacheVehicleLocationData[termInfo.CarNumber] = new(cacheVehicleData)
	}
	// 更新缓存数据
	cacheData := cacheVehicleLocationData[termInfo.CarNumber]
	cacheData.Speed = int(speed)
	cacheData.TimeStamp = time.Now().Unix()

	// 根据缓存判定保存定位数据
	if saveTag {
		fmt.Println("保存位置数据:", termInfo.CarNumber, longStr, latStr, speed, direction, uploadTime)

	}

	// 平台通用回复
	body := &DefServerUniversalReply{
		ReplySerialNum: request.Head.SerialNum,
		ReplyID:        "0200",
		Result:         ServerUniversalReplyStatus[handleState],
	}
	ServerUniversalReply(conn, body)

	return
}

// MultiUploadLocation 批量位置信息上传 0x0704
func MultiUploadLocation(request *ProtocolData, conn net.Conn) {

	body := &DefServerUniversalReply{
		ReplySerialNum: request.Head.SerialNum,
		ReplyID:        "0704",
		Result:         ServerUniversalReplyStatus["成功"],
	}
	ServerUniversalReply(conn, body)
	return

}

// QueryLocation 位置信息查询 0x8201
func QueryLocation() {

	for key, val := range OnlineClient {
		log.Printf("[位置信息查询]向: %s 发送查询指令", key)
		conn := val.Conn

		signal := make(chan int)
		ctx, cancelFunc := context.WithTimeout(context.Background(), time.Second*time.Duration(10))
		go SendMsgToTerm(signal, conn, "8201", nil)
		for {
			select {
			case <-ctx.Done():
				log.Printf("[位置信息查询]连接地址:%s 查询消息发送超时", conn.RemoteAddr().String())
				cancelFunc()
				return
			case <-signal:
				cancelFunc()
				return
			}
		}
	}

}

// QueryLocationReply 位置信息查询应答 0x0201
func QueryLocationReply(request *ProtocolData, conn net.Conn) {
	log.Printf("[查询位置应答] 连接: %s 数据处理未实现...\n", conn.RemoteAddr().String())
	log.Println("[查询位置应答数据]", request.Body)
	body := &DefServerUniversalReply{
		ReplySerialNum: request.Head.SerialNum,
		ReplyID:        "0201",
		Result:         ServerUniversalReplyStatus["成功"],
	}
	ServerUniversalReply(conn, body)
	return
}
