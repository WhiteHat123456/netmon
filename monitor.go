package main

/*
#include <stdio.h>
#include <sys/socket.h>
#include <sys/ioctl.h>
#include <netinet/in.h>
#include <linux/sockios.h>
#include <linux/if.h>
#include <linux/ethtool.h>
#include <string.h>
#include <stdlib.h>
#include <unistd.h>

int get_interface_speed(char *ifname){
    int sock;
    struct ifreq ifr;
    struct ethtool_cmd edata;
    int rc;
    sock = socket(AF_INET, SOCK_STREAM, 0);
    // string copy first argument into struct
    strncpy(ifr.ifr_name, ifname, sizeof(ifr.ifr_name));
    ifr.ifr_data = &edata;
    // set some global options from ethtool API
    edata.cmd = ETHTOOL_GSET;
    // issue ioctl
    rc = ioctl(sock, SIOCETHTOOL, &ifr);

    close(sock);

    if (rc < 0) {
        perror("ioctl");        // lets not error out here
        // make sure to zero out speed
        return 0;
    }

    return edata.speed;
}
*/
import "C"

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"time"
	"unsafe"

	"github.com/gin-gonic/gin"
	cors "github.com/rs/cors/wrapper/gin"
)

const IFCEPATH = "/sys/class/net/"
const WSNAME = "wondershaperscript.sh"
const WSSYSPATH = "/usr/local/bin/wsssh"

var BASELIM int64 = 8 * 1024
var UPPERLIM int64 = 80 * 1024
var THRESHOLDLIM int64 = 45
var PUNISHMENT int64 = 10
var PUNISHLIM int64 = 8 * 1024

var IFCEPREFIX = ""
var QOS = false
var PASSWD = "netmon20"

type Ifce struct {
	Name      string `json:"name,omitempty"`
	Tx        int64  `json:"tx,omitempty"`
	Rx        int64  `json:"rx,omitempty"`
	TxPackage int64  `json:"tx_package,omitempty"`
	RxPackage int64  `json:"rx_package,omitempty"`

	TxRate        int64 `json:"tx_rate,omitempty"`
	RxRate        int64 `json:"rx_rate,omitempty"`
	TxPackageRate int64 `json:"tx_package_rate,omitempty"`
	RxPackageRate int64 `json:"rx_package_rate,omitempty"`

	Credit int64 `json:"credit,omitempty"`
	Banned bool  `json:"banned,omitempty"`
	White  bool  `json:"white,omitempty"`
}

type Ifces struct {
	Interfaces map[string]*Ifce `json:"interfaces,omitempty"`
	Timestamp  int64            `json:"timestamp,omitempty"`
	Interval   int64            `json:"interval,omitempty"`
}

type DB struct {
	Ifces  Ifces            `json:"ifces,omitempty"`
	LimMap map[string]bool  `json:"limmap,omitempty"`
	CreMap map[string]int64 `json:"cremap,omitempty"`
	WhiMap map[string]bool  `json:"whimap,omitempty"`
}

type BanPost struct {
	Name   string `form:"name" json:"name" binding:"required"`
	Banned int    `form:"banned" json:"banned" binding:"required"`
}

var interfaces Ifces
var logLevel int64 = 3
var interval int64 = 3
var limitMap map[string]bool
var whiteMap map[string]bool
var creditMap map[string]int64

func Error(err error) {
	fmt.Println("[Error]", err.Error())
}

func Log(level int64, i ...interface{}) {
	if logLevel >= level {
		fmt.Print("[NetMon] ")
		fmt.Println(i...)
	}
}

// Will be returned by x Kbps (original x MB/s)
func Speed(name string) int64 {
	ifname := []byte(name+"\x00")
	sp := C.get_interface_speed((*C.char)(unsafe.Pointer(&ifname[0])))
	return int64(sp) * int64(1024 * 8)
}

const DBSAVE = 1
const DBLOAD = -1

func db(action int) {
	db := "/tmp/.netmon.json"
	if action == DBLOAD {
		var dbi DB
		if b, err := ioutil.ReadFile(db); err == nil {
			json.Unmarshal(b, &dbi)
			interfaces = dbi.Ifces
			limitMap = dbi.LimMap
			creditMap = dbi.CreMap
			whiteMap = dbi.WhiMap
			count := 0
			if interfaces.Interfaces != nil {
				for key := range interfaces.Interfaces {
					if !strings.HasPrefix(key, IFCEPREFIX) {
						delete(interfaces.Interfaces, key)
					}
				}
				count = len(interfaces.Interfaces)
			}
			Log(0, "Loading history data success", count)
		} else {
			Log(0, "Error loading history data")
		}
	} else {
		dbi := DB{
			Ifces:  interfaces,
			LimMap: limitMap,
			CreMap: creditMap,
			WhiMap: whiteMap,
		}
		b, err := json.Marshal(dbi)
		if err == nil {
			ioutil.WriteFile(db, b, 0644)
		}
	}
}

func readFileOrEmpty(path string) int64 {
	dat, err := ioutil.ReadFile(path)
	if err != nil {
		Error(err)
		return -1
	}
	data, err := strconv.ParseInt(strings.TrimSpace(string(dat)), 10, 64)
	if err != nil {
		return -1
	}
	return data
}

// Return KBit
func readStatsOrEmpty(name string, file string) int64 {
	return readFileOrEmpty(fmt.Sprintf("%s%s/statistics/%s", IFCEPATH, name, file)) / int64(128)
}

func UpdateAllInterfaces() {
	intv64 := int64(interval)
	var totalTx int64 = 0
	t1 := time.Now().UnixNano()
	files, err := ioutil.ReadDir(IFCEPATH)
	if err != nil {
		Error(err)
	} else {
		for _, f := range files {
			name := f.Name()
			if strings.HasPrefix(name, IFCEPREFIX) {
				ifce := Ifce{
					Name:      name,
					Tx:        readStatsOrEmpty(name, "tx_bytes"),
					Rx:        readStatsOrEmpty(name, "rx_bytes"),
					TxPackage: readStatsOrEmpty(name, "tx_packets"),
					RxPackage: readStatsOrEmpty(name, "rx_packets"),
					Credit:    creditMap[name],
					Banned:    limitMap[name],
					White:     whiteMap[name],
				}
				if i, ok := interfaces.Interfaces[name]; ok && i != nil {
					ifce.RxRate = (ifce.Rx - i.Rx) / intv64
					ifce.TxRate = (ifce.Tx - i.Tx) / intv64
					ifce.RxPackageRate = (ifce.RxPackage - i.RxPackage) / intv64
					ifce.TxPackageRate = (ifce.TxPackage - i.TxPackage) / intv64
					totalTx += ifce.TxRate
					delete(interfaces.Interfaces, name)
				}
				interfaces.Interfaces[name] = &ifce
			}
		}
	}
	t2 := time.Now().UnixNano()
	interfaces.Timestamp = t2
	if QOS && totalTx >= UPPERLIM {
		for _, ifce := range interfaces.Interfaces {
			qos(totalTx, ifce)
		}
	}
	db(DBSAVE)
	Log(5, t2, "Updated", t2-t1)
}

func qos(totalTx int64, i *Ifce) {
	if i.TxRate <= BASELIM {
		return
	}
	// var percentage float32 = float32(i.TxRate) / float32(totalTx)
	nRate := Speed(i.Name)
	if nRate <= 0 {
		nRate = totalTx
	}
	var percentage float32 = float32(i.TxRate) / float32(nRate)
	if percentage < 0.3 {
		creditMap[i.Name] -= 1
		return
	}
	if limitMap[i.Name] || whiteMap[i.Name] {
		return
	}
	if percentage > 0.65 {
		creditMap[i.Name] += 1
	}
	if percentage > 0.4 {
		creditMap[i.Name] += 1
		Log(4, "Warn", i.Name, creditMap[i.Name])
	}

	if creditMap[i.Name]*int64(interval) > THRESHOLDLIM {
		limitMap[i.Name] = true
		execWS(i.Name, PUNISHLIM)
		go func() {
			time.Sleep(time.Duration(PUNISHMENT) * time.Minute)
			creditMap[i.Name] = 0
			limitMap[i.Name] = false
			execWS(i.Name, 0)
		}()
	}
}

func InitServer() *gin.Engine {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()

	r.Use(gin.Recovery())
	r.Use(cors.Default())

	authorized := r.Group("/", gin.BasicAuth(gin.Accounts{
		"netmon": PASSWD,
	}))
	authorized.GET("/stats", func(c *gin.Context) {
		c.JSON(200, interfaces)
	})
	authorized.StaticFS("/web", AssetFile())
	authorized.POST("/ban", func(c *gin.Context) {
		var json BanPost
		if err := c.ShouldBindJSON(&json); err != nil {
			c.JSON(400, gin.H{"error": err.Error()})
		} else {
			if json.Banned > 0 {
				whiteMap[json.Name] = false
				limitMap[json.Name] = true
				execWS(json.Name, PUNISHLIM)
			} else {
				if json.Banned < -10 {
					whiteMap[json.Name] = true
				}
				creditMap[json.Name] = 0
				limitMap[json.Name] = false
				execWS(json.Name, 0)
			}
			db(DBSAVE)
			c.JSON(200, gin.H{"banned": limitMap[json.Name]})
		}
	})

	return r
}

func StartMonitor() {
	interfaces.Interval = interval
	if interfaces.Interfaces == nil {
		interfaces.Interfaces = make(map[string]*Ifce)
	}
	if limitMap == nil {
		limitMap = make(map[string]bool)
	}
	if creditMap == nil {
		creditMap = make(map[string]int64)
	}
	if whiteMap == nil {
		whiteMap = make(map[string]bool)
	}

	for {
		go func() {
			UpdateAllInterfaces()
			if r := recover(); r != nil {
				var err error
				switch x := r.(type) {
				case string:
					err = errors.New(x)
				case error:
					err = x
				default:
					err = errors.New("Unknown Error")
				}
				fmt.Println("Uncaught Error", err.Error())
			}
		}()
		time.Sleep(time.Duration(interval) * time.Second)
	}
}

func execWS(ifname string, rate int64) bool {
	nRate := Speed(ifname)
        if nRate <= 0 {
                nRate = UPPERLIM
        }
	args := []string{WSSYSPATH, "-c", "-a", ifname}
	exec.Command("/bin/bash", args...).Output()
	if rate <= 0 {
		Log(1, "Unban", ifname, nRate)
		rates := fmt.Sprintf("%d", nRate)
		args = []string{WSSYSPATH, "-a", ifname, "-d", rates, "-u", rates}
	} else {
		if rate < 100 {
			nRate = nRate / 100 * rate
		} else {
			nRate = rate
		}
		Log(1, "Ban", ifname, nRate)
		rates := fmt.Sprintf("%d", rate)
		args = []string{WSSYSPATH, "-a", ifname, "-d", rates, "-u", rates}
	}
	out, err := exec.Command("/bin/bash", args...).Output()
	if err != nil {
		Log(3, string(out))
		return false
	}
	return true
}

func execCMD(cmd string) string {
	args := strings.Split(cmd, " ")
	for i, _ := range args {
		args[i] = strings.ReplaceAll(args[i], "//", " ")
	}
	outByte, err := exec.Command(args[0], args[1:]...).Output()
	out := "[Out]\n" + string(outByte)
	if err != nil {
		out = "[Error]\n" + err.Error() + "\n" + out
	}
	return out
}

func sysCheck() {
	_, err := os.Stat(WSSYSPATH)
	if err == nil || os.IsExist(err) {
		return
	}
	rawBin, _ := Asset(WSNAME)
	if ioutil.WriteFile(WSSYSPATH, rawBin, 0777) != nil {
		fmt.Println("[Error]", "Fail to extend file, please run as root!")
		os.Exit(0)
	}
}

func main() {

	port := int64(8358)

	flag.Int64Var(&interval, "intv", 3, "Updating interval (Second: 3s)")
	flag.Int64Var(&logLevel, "log", 3, "Log level (Number: 3)")
	flag.StringVar(&IFCEPREFIX, "ifce", "", "Network interface prefix (String: null)")
	flag.Int64Var(&port, "port", 8358, "Binding port (Number: 8358)")
	flag.BoolVar(&QOS, "qos", false, "Run QOS (Bool: false)")
	flag.StringVar(&PASSWD, "pass", "netmon20", "Password of panel [Username: netmon] (String: netmon20)")

	flag.Int64Var(&BASELIM, "min", 8*1024, "Minimun scale of bandwidth (KBit: 8Mbps)")
	flag.Int64Var(&UPPERLIM, "max", 80*1024, "Maximum scale of bandwidth (KBit: 80Mbps)")
	flag.Int64Var(&THRESHOLDLIM, "time", 45, "Machine overrun exceeds this period of time will receive a limit (Second: 45s)")
	flag.Int64Var(&PUNISHMENT, "ptime", 10, "Overrun machine will be limited for this long (Minute: 10m)")
	flag.Int64Var(&PUNISHLIM, "limit", -1, "Limit the bandwidth to (Minute: Min)")

	flag.Parse()
	if PUNISHLIM <= 0 {
		PUNISHLIM = BASELIM
	}

	Log(0, "Using Log Level", logLevel)
	Log(0, "Using Port", port)
	Log(0, "Using Prefix", IFCEPREFIX)
	Log(0, "Using QOS", QOS)

	sysCheck()
	db(DBLOAD)

	r := InitServer()
	go StartMonitor()

	sport := fmt.Sprintf(":%d", port)
	Log(0, "Server Started")
	r.Run(sport)
}
