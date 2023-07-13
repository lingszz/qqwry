package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"net"
	"os"

	"golang.org/x/text/encoding/simplifiedchinese"
)

var A []byte
var B []byte

func FindFlag(Ip uint32, file []byte) uint32 {
	var flag = []byte{0x00, 0x00, 0x00, 0x00}
	fastFlag := binary.LittleEndian.Uint32(file[0:4])
	lastFlag := binary.LittleEndian.Uint32(file[4:8])
	for {
		mid := (lastFlag - fastFlag) / 7 / 2
		mid = mid*7 + fastFlag
		if binary.LittleEndian.Uint32(file[mid : mid+7][0:4]) == Ip {
			copy(flag, file[mid : mid+7][4:7])
			ipIndex := binary.LittleEndian.Uint32(flag)
			return ipIndex
		} else if binary.LittleEndian.Uint32(file[mid : mid+7][0:4]) > Ip {
			lastFlag = mid
		} else if binary.LittleEndian.Uint32(file[mid : mid+7][0:4]) < Ip {
			fastFlag = mid
		}
		if lastFlag-fastFlag <= 7 {
			if binary.LittleEndian.Uint32(file[lastFlag : lastFlag+7][0:4]) <= Ip {
				copy(flag, file[lastFlag : lastFlag+7][4:7])
			} else {
				copy(flag, file[fastFlag : fastFlag+7][4:7])
			}
			ipIndex := binary.LittleEndian.Uint32(flag)
			return ipIndex
		}
	}
}

func GetData(flagIndex uint32, file []byte, k uint32) {
	var OffsetFlag = []byte{0x00, 0x00, 0x00, 0x00}
	var i uint32
	var j uint32
	if file[flagIndex] == 0x01 {
		copy(OffsetFlag, file[flagIndex+1:flagIndex+4])
		GetData(binary.LittleEndian.Uint32(OffsetFlag), file, 0)
	} else if file[flagIndex] == 0x02 {
		copy(OffsetFlag, file[flagIndex+1:flagIndex+4])
		GetData(binary.LittleEndian.Uint32(OffsetFlag), file, 1)
		if file[flagIndex+4] == 0x01 || file[flagIndex+4] == 0x02 {
			copy(OffsetFlag, file[flagIndex+5:flagIndex+8])
			GetData(binary.LittleEndian.Uint32(OffsetFlag), file, 1)
		} else {
			GetData(flagIndex+4, file, 1)
		}
	} else {
		i = 0
		for {
			if file[flagIndex+i] == 0x00 {
				if len(A) == 0 {
					A = file[flagIndex : flagIndex+i]
				} else if len(B) == 0 {
					B = file[flagIndex : flagIndex+i]
				}
				break
			}
			i++
		}
		if k == 0 {
			if file[flagIndex+i+1] == 0x01 || file[flagIndex+i+1] == 0x02 {
				GetData(flagIndex+i+1, file, 1)
			} else {
				j = flagIndex + i + 1
				i = 0
				for {
					if file[j+i] == 0x00 {
						if len(A) == 0 {
							A = file[j : j+i]
						} else if len(B) == 0 {
							B = file[j : j+i]
						}
						break
					}
					i++
				}
			}
		}
	}
	// fmt.Println("------")
	// fmt.Printf("A: %v\n", A)
	// fmt.Printf("B: %v\n", B)
	// fmt.Printf("file[flagIndex]: %v\n", file[flagIndex])
}

func main() {
	file, err := os.ReadFile("qqwry.dat")
	if err != nil {
		log.Panic(err)
	}
	// findFlag("8.8.8.8")
	ip := net.ParseIP("255.255.255.0")
	ip = ip.To4()
	// fmt.Printf("FindFlag(binary.BigEndian.Uint32(ip), file): %v\n", FindFlag(binary.BigEndian.Uint32(ip), file))
	GetData(FindFlag(binary.BigEndian.Uint32(ip), file)+4, file, 0)
	decoder := simplifiedchinese.GB18030.NewDecoder()
	AgbkString, _ := decoder.Bytes(A)
	BgbkString, _ := decoder.Bytes(B)
	// fmt.Printf("A: %v\n", A)
	fmt.Printf("string(AgbkString): %v\n", string(AgbkString))
	fmt.Printf("B: %v\n", B)
	fmt.Printf("string(BgbkString): %v\n", string(BgbkString))
}
