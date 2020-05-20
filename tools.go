package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/imroc/biu"
)

// encodeBCD BCD编码
func encodeBCD(d string) []byte {
	var result []byte
	dSlice := strings.Split(d, "")
	l := len(dSlice)

	for i := 0; i < l; i += 2 {
		d1, err := strconv.Atoi(dSlice[i])
		d2, err := strconv.Atoi(dSlice[i+1])
		d1bin, err := dec2x(d1, 2)
		if len(d1bin) < 4 {
			d1bin = strJoin("", strings.Repeat("0", 4-len(d1bin)), d1bin)
		}
		d2bin, err := dec2x(d2, 2)
		if len(d2bin) < 4 {
			d2bin = strJoin("", strings.Repeat("0", 4-len(d2bin)), d2bin)
		}
		if err != nil {
			panic(err)
		}
		result = append(result, biu.BinaryStringToBytes(strJoin("", d1bin, d2bin))...)
	}
	if len(result) != 6 {
		panic("BCD编码失败,长度不为6")
	}
	return result
}

// decodeBCD BCD解码
func decodeBCD(d []byte) string {
	l := len(d)

	var fullBinStr []string
	for i := 0; i < l; i++ {
		ts := biu.ToBinaryString(d[i])
		fullBinStr = append(fullBinStr, ts)
	}
	if fullBinStr == nil {
		return ""
	}

	a := len(fullBinStr)

	var res string
	for i := 0; i < a; i++ {
		tmpBinStr := fullBinStr[i]

		s1 := strconv.Itoa(binStr2DEC(tmpBinStr[:4]))
		s2 := strconv.Itoa(binStr2DEC(tmpBinStr[4:]))

		if s1 == "0" && len(s2) >= 2 {
			s1 = ""
		}
		res += s1 + s2
	}

	return res
}

// dec2HexByte 十进制转十六进制字节类型
func dec2HexByte(n int, l int) ([]byte, error) {
	// 一个byte 最大 255(ff)
	// 所以 l 不可能为奇数
	if l%2 != 0 {
		return nil, errors.New("l 不为偶数")
	}
	// 格式化十六进制字符串
	hexStr := fmt.Sprintf("%x", n)
	// 补全
	if len(hexStr) != l {
		rpn := l - len(hexStr)
		hexStr = strJoin("", strings.Repeat("0", rpn)+hexStr)
	}
	// 十六进制字符串转字节
	b, err := hex.DecodeString(hexStr)
	if err != nil {
		return nil, err
	}
	return b, nil
}

// strJoin 字符串拼接
func strJoin(sep string, args ...string) string {
	l := len(args)
	lastIndex := l - 1
	var buff bytes.Buffer
	for i := 0; i < l; i++ {
		buff.WriteString(args[i])
		if i != lastIndex {
			buff.WriteString(sep)
		}
	}
	return buff.String()
}

// dec2x 给定数字转换到 2 8 16 进制
func dec2x(n, flag int) (string, error) {
	if n < 0 {
		return strconv.Itoa(n), errors.New("只支持正整数")
	}
	if flag != 2 && flag != 8 && flag != 16 {
		return strconv.Itoa(n), errors.New("只支持二、八、十六进制的转换")
	}
	result := ""
	h := map[int]string{
		0:  "0",
		1:  "1",
		2:  "2",
		3:  "3",
		4:  "4",
		5:  "5",
		6:  "6",
		7:  "7",
		8:  "8",
		9:  "9",
		10: "A",
		11: "B",
		12: "C",
		13: "D",
		14: "E",
		15: "F",
	}
	for ; n > 0; n /= flag {
		lsb := h[n%flag]
		result = lsb + result
	}
	return result, nil
}

// binStr2DEC 二进制字符串转十进制
func binStr2DEC(s string) (num int) {
	l := len(s)
	for i := l - 1; i >= 0; i-- {
		num += (int(s[l-i-1]) & 0xf) << uint8(i)
	}
	return
}
