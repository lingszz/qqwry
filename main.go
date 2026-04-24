package main

import (
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"github.com/gin-gonic/gin"
	"golang.org/x/text/encoding/simplifiedchinese"
)

// IPLocation 存储IP地址的位置信息
type IPLocation struct {
	IP      string `json:"ip" xml:"ip"`
	Country string `json:"country" xml:"country"`
	Area    string `json:"area" xml:"area"`
}

// IPQuery 查询器
type IPQuery struct {
	mu   sync.RWMutex
	data []byte
}

// NewIPQuery 创建新的IP查询器
func NewIPQuery(dataFile string) (*IPQuery, error) {
	data, err := os.ReadFile(dataFile)
	if err != nil {
		return nil, fmt.Errorf("failed to read data file: %w", err)
	}
	return &IPQuery{
		data: data,
	}, nil
}

// FindFlag 查找IP在索引中的位置
func (q *IPQuery) FindFlag(ip uint32) uint32 {
	file := q.data
	flag := []byte{0x00, 0x00, 0x00, 0x00}
	fastFlag := binary.LittleEndian.Uint32(file[0:4])
	lastFlag := binary.LittleEndian.Uint32(file[4:8])

	for {
		mid := (lastFlag - fastFlag) / 7 / 2
		mid = mid*7 + fastFlag
		
		currentIP := binary.LittleEndian.Uint32(file[mid : mid+7][0:4])
		if currentIP == ip {
			copy(flag, file[mid:mid+7][4:7])
			return binary.LittleEndian.Uint32(flag)
		} else if currentIP > ip {
			lastFlag = mid
		} else {
			fastFlag = mid
		}

		if lastFlag-fastFlag <= 7 {
			if binary.LittleEndian.Uint32(file[lastFlag:lastFlag+7][0:4]) <= ip {
				copy(flag, file[lastFlag:lastFlag+7][4:7])
			} else {
				copy(flag, file[fastFlag:fastFlag+7][4:7])
			}
			return binary.LittleEndian.Uint32(flag)
		}
	}
}

// GetData 获取IP对应的地理数据
func (q *IPQuery) GetData(flagIndex uint32, result *[2][]byte, mode uint32) {
	file := q.data
	var offsetFlag = []byte{0x00, 0x00, 0x00, 0x00}
	var i, j uint32

	if file[flagIndex] == 0x01 {
		copy(offsetFlag, file[flagIndex+1:flagIndex+4])
		q.GetData(binary.LittleEndian.Uint32(offsetFlag), result, 0)
	} else if file[flagIndex] == 0x02 {
		copy(offsetFlag, file[flagIndex+1:flagIndex+4])
		q.GetData(binary.LittleEndian.Uint32(offsetFlag), result, 1)
		if file[flagIndex+4] == 0x01 || file[flagIndex+4] == 0x02 {
			copy(offsetFlag, file[flagIndex+5:flagIndex+8])
			q.GetData(binary.LittleEndian.Uint32(offsetFlag), result, 1)
		} else {
			q.GetData(flagIndex+4, result, 1)
		}
	} else {
		// 读取第一个字符串
		i = 0
		for {
			if file[flagIndex+i] == 0x00 {
				if len(result[0]) == 0 {
					result[0] = file[flagIndex : flagIndex+i]
				} else if len(result[1]) == 0 {
					result[1] = file[flagIndex : flagIndex+i]
				}
				break
			}
			i++
		}

		if mode == 0 {
			if file[flagIndex+i+1] == 0x01 || file[flagIndex+i+1] == 0x02 {
				q.GetData(flagIndex+i+1, result, 1)
			} else {
				j = flagIndex + i + 1
				i = 0
				for {
					if file[j+i] == 0x00 {
						if len(result[0]) == 0 {
							result[0] = file[j : j+i]
						} else if len(result[1]) == 0 {
							result[1] = file[j : j+i]
						}
						break
					}
					i++
				}
			}
		}
	}
}

// Query 查询IP地址信息
func (q *IPQuery) Query(ipStr string) (*IPLocation, error) {
	q.mu.RLock()
	defer q.mu.RUnlock()

	ip := net.ParseIP(ipStr)
	if ip == nil {
		return nil, fmt.Errorf("invalid IP address: %s", ipStr)
	}

	ip = ip.To4()
	if ip == nil {
		return nil, fmt.Errorf("only IPv4 is supported")
	}

	ipUint32 := binary.BigEndian.Uint32(ip)
	flagIndex := q.FindFlag(ipUint32) + 4
	
	var result [2][]byte
	q.GetData(flagIndex, &result, 0)

	decoder := simplifiedchinese.GB18030.NewDecoder()
	country, _ := decoder.Bytes(result[0])
	area, _ := decoder.Bytes(result[1])

	return &IPLocation{
		IP:      ipStr,
		Country: string(country),
		Area:    string(area),
	}, nil
}

// setupRouter 配置路由
func setupRouter(query *IPQuery) *gin.Engine {
	router := gin.Default()

	// JSON 接口
	router.GET("/ipsearch/json", func(c *gin.Context) {
		ip := c.Query("ip")
		if ip == "" {
			c.JSON(http.StatusBadRequest, gin.H{"error": "ip parameter is required"})
			return
		}

		location, err := query.Query(ip)
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		c.JSON(http.StatusOK, location)
	})

	// XML 接口
	router.GET("/ipsearch/xml", func(c *gin.Context) {
		ip := c.Query("ip")
		if ip == "" {
			c.XML(http.StatusBadRequest, gin.H{"error": "ip parameter is required"})
			return
		}

		location, err := query.Query(ip)
		if err != nil {
			c.XML(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		// 构造XML响应，使其可以被FILTERXML函数解析
		xmlData := struct {
			XMLName xml.Name `xml:"root"`
			Country string   `xml:"country"`
			Area    string   `xml:"area"`
		}{
			Country: location.Country,
			Area:    location.Area,
		}

		c.XML(http.StatusOK, xmlData)
	})

	// 健康检查接口
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	return router
}

func main() {
	// 初始化IP查询器
	query, err := NewIPQuery("qqwry.dat")
	if err != nil {
		log.Fatalf("Failed to initialize IP query: %v", err)
	}

	// 设置路由
	router := setupRouter(query)

	// 启动服务
	addr := ":18081"
	fmt.Printf("Server starting on %s\n", addr)
	fmt.Printf("Usage examples:\n")
	fmt.Printf("  JSON: http://127.0.0.1:18081/ipsearch/json?ip=8.8.8.8\n")
	fmt.Printf("  XML:  http://127.0.0.1:18081/ipsearch/xml?ip=8.8.8.8\n")
	fmt.Printf("  Excel: =FILTERXML(WEBSERVICE(\"http://127.0.0.1:18081/ipsearch/xml?ip=\" & C2), \"//\")\n")

	if err := router.Run(addr); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}