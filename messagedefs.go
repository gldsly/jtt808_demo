/*
jt/t808 消息相关数据定义
*/
package main

var (
	// LicensePlateColorMap 车牌颜色定义
	LicensePlateColorMap = map[int]string{
		0: "无颜色/保留/默认",
		1: "蓝色",
		2: "黄色",
		3: "黑色",
		4: "白色",
		9: "其他",
	}
	// RegistrationReplyStatus 终端注册,结果映射
	RegistrationReplyStatus = map[string]string{
		"成功":       "00",
		"车辆已被注册":   "01",
		"数据库中无该车辆": "02",
		"终端已被注册":   "03",
		"数据库中无该终端": "04",
	}
	// ServerUniversalReplyStatus 平台通用应答
	ServerUniversalReplyStatus = map[string]string{
		"成功":     "00",
		"失败":     "01",
		"消息有误":   "02",
		"不支持":    "03",
		"报警处理确认": "04",
	}
	// locationAdditionalMsgIDMap 位置附加信息ID映射表
	locationAdditionalMsgIDMap = map[string]string{
		"01": "里程",
		"02": "油量",
		"03": "行驶记录功能获取的速度",
		"04": "需要人工确认报警事件的 ID",
		"11": "超速报警附加信息",
		"12": "进出区域/路线报警附加信息",
		"13": "路段行驶时间不足/过长报警附加信息",
		"25": "扩展车辆信号状态位",
		"2A": "IO状态位",
		"2B": "模拟量",
		"30": "无线通信网络信号强度",
		"31": "GNSS定位卫星数",
		"E0": "自定义信息长度",
	}
	// cacheVehicleLocationData 缓存车辆位置数据
	cacheVehicleLocationData = make(map[string]*cacheVehicleData)
)

type cacheVehicleData struct {
	Speed     int
	PerState  bool
	TimeStamp int64
}

// DefTerminalUniversalReply 终端通用应答 0x0001
type DefTerminalUniversalReply struct {
	ReplySerialNum string `comment:"对应平台消息流水号"`
	ReplyID        string `comment:"应答ID,对应平台消息ID"`
	Result         string `comment:"结果: 0 成功/确认 1 失败 2 消息有误 3 不支持"`
}

// DefServerUniversalReply 平台通用应答 0x8001
type DefServerUniversalReply struct {
	ReplySerialNum uint16 `comment:"对应终端消息流水号"`
	ReplyID        string `comment:"应答ID,对应终端消息ID"`
	Result         string `comment:"结果: 0 成功/确认 1 失败 2 消息有误 3 不支持 4 报警处理确认"`
}

// DefTerminalKeepAlive 终端心跳 0x0002
type DefTerminalKeepAlive struct {
}

// DefTerminalRegistration 终端注册 0x0100
type DefTerminalRegistration struct {
	ProvinceID            int    `comment:"省域ID,2字节,默认为0保留,GB/2260中规定的行政区划分代码六位中前两位"`
	CityID                int    `comment:"市/县域ID,2字节,默认为0保留,GB/2260中规定的行政区划分代码六位中后四位"`
	ManufacturerID        string `comment:"终端制造商编码,5字节"`
	TerminalType          string `comment:"终端型号,20字节,位数不足时,后补0X00"`
	TerminalID            string `comment:"终端ID,7字节,位数不足时,后补0X00"`
	LicensePlateColor     string `comment:"车牌颜色,1字节,按照JT/T415-2006 的 5.4.12 未上牌取值为0"`
	VehicleIdentification string `comment:"车辆标识,字符串,颜色为0时,标识车辆VIN(车架号码),否则为车牌号"`
}

// DefTerminalRegistrationReply 终端注册应答 0x8100
type DefTerminalRegistrationReply struct {
	ReplySerialNum uint16 `comment:"对应终端注册消息流水号"`
	Result         string `comment:"结果: 0 成功 1 车辆已被注册 2 数据库中无该车辆 3 终端已被注册 4 数据库中无该终端"`
	AuthCode       string `comment:"只有注册成功时,存在该字段"`
}

// DefTerminalAuth 终端鉴权 0x0102
type DefTerminalAuth struct {
	AuthCode string `comment:"终端重连后,上报鉴权码"`
}

// DefUploadLocation 位置信息汇报
type DefUploadLocation struct {
	AlarmTag   string `comment:"4字节(32位)二进制字符串,每一位代表一种报警,此值变化后终端立即上报位置"`
	State      string `comment:"4字节(32位)二进制字符串,每一位代表一种车辆状态,此值变化后终端立即上报位置"`
	Latitude   string `comment:"纬度, 以度为单位的纬度值乘以10的6次方，精确到百万 分之一度"`
	Longitude  string `comment:"经度, 以度为单位的经度值乘以10的6次方，精确到百万 分之一度"`
	Elevation  int    `comment:"海拔高度，单位为米(m)"`
	Speed      int    `comment:"车辆速度,单位 km/h"`
	Direction  int    `comment:"方向,方位角,范围 0-359 正北为 0 顺时针计算"`
	UploadTime string `comment:"上传时间,BCD解码6个字节"`
}

// DefQueryLocationReply 位置信息查询响应
type DefQueryLocationReply struct {
}
