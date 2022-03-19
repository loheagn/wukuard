package main

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"net"
	"os"
	"os/exec"
	"regexp"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	pb "github.com/loheagn/wukuard/grpc"
	"google.golang.org/grpc"
	"gopkg.in/ini.v1"
)

type InterfaceConf struct {
	PrivateKey string
	Address    string
	ListenPort int32
	PostUp     string
	PreDown    string
}

type PeerConf struct {
	PublicKey           string
	AllowedIPs          string
	Endpoint            string
	PersistentKeepalive int32
}

type WgConf struct {
	interfaceConf *InterfaceConf
	peerConfList  []*PeerConf
}

var (
	serverIP       string
	serverGrpcPort string
	interfaceName  string // the name of network interface
)

var wgMutex sync.Mutex

func (interfaceConf InterfaceConf) generateString() string {
	return fmt.Sprintf(`
[Interface]
PrivateKey = %s
Address = %s
ListenPort = %d
PostUp = %s
PreDown = %s

`, interfaceConf.PrivateKey, interfaceConf.Address, interfaceConf.ListenPort, interfaceConf.PostUp, interfaceConf.PreDown)
}

func (peerConf PeerConf) generateString() string {
	return fmt.Sprintf(`
[Peer]
Publickey = %s
AllowedIPs = %s
Endpoint = %s
PersistentKeepalive = %d

`, peerConf.PublicKey, peerConf.AllowedIPs, peerConf.Endpoint, peerConf.PersistentKeepalive)
}

func (wgConf WgConf) generateString() string {
	buf := bytes.Buffer{}
	buf.WriteString(wgConf.interfaceConf.generateString())
	for _, v := range wgConf.peerConfList {
		buf.WriteString(v.generateString())
	}
	return buf.String()
}

const (
	basePath     = "/etc/wireguard/"
	confFilename = "/etc/wireguard/wukuard.conf"
	serviceName  = "wg-quick@wukuard.service"
)

func compareInterfaceConf(src, input *InterfaceConf) bool {
	if src == nil || input == nil {
		return src == input
	}
	if src.PrivateKey != input.PrivateKey ||
		src.Address != input.Address ||
		src.ListenPort != input.ListenPort ||
		src.PostUp != input.PostUp ||
		src.PreDown != input.PreDown {
		return false
	}
	return true
}

func getCurrentConf() (*InterfaceConf, string, error) {
	wholeConfStr, err := readFile(confFilename)
	if err != nil {
		return nil, "", err
	}
	cfg, err := ini.Load(confFilename)
	if err != nil {
		return nil, "", err
	}
	interfaceConf := &InterfaceConf{}
	err = cfg.Section("Interface").MapTo(interfaceConf)
	if err != nil {
		return nil, "", err
	}
	return interfaceConf, wholeConfStr, nil
}

func checkServiceIsRunning() bool {
	output, err := exec.Command("systemctl", "status", serviceName).Output()
	if err != nil {
		return false
	}
	matched, err := regexp.MatchString("Active: active", string(output))
	return err == nil && matched
}

func prepareConfFile() error {
	syscall.Umask(0022)
	if _, err := os.Stat(basePath); os.IsNotExist(err) {
		err = os.MkdirAll(basePath, os.ModePerm)
		if err != nil {
			return err
		}
	}
	// generate the wg conf
	if _, err := os.Stat(confFilename); os.IsNotExist(err) {
		confFile, err := os.Create(confFilename)
		if err != nil {
			return err
		}
		_ = confFile.Close()
	} else {
		return err
	}
	return nil
}

func getLocalIP() string {
	conn, err := net.Dial("udp", serverIP)
	if err != nil {
		log.Fatal(err)
	}
	defer conn.Close()

	localAddr := conn.LocalAddr().(*net.UDPAddr)

	return localAddr.IP.String()
}

func getMacAddress() string {
	if interfaceName == "" {
		return ""
	}
	ifas, err := net.Interfaces()
	if err != nil {
		return ""
	}
	for _, v := range ifas {
		if v.Name == interfaceName {
			return v.HardwareAddr.String()
		}
	}
	return ""
}

func getHostname() string {
	hostname, err := os.Hostname()
	if err != nil {
		return ""
	}
	return hostname
}

func parseServerAddr(addr string) {
	ip, port, err := net.SplitHostPort(addr)
	if err != nil {
		log.Panic(err)
	}
	serverIP, serverGrpcPort = ip, port
}

func mapGrpcResponse(network *pb.NetWorkResponse) *WgConf {
	wgConf := &WgConf{}
	interfaceResponse := network.GetInterfaceResponse()
	if interfaceResponse == nil {
		return nil
	}
	wgConf.interfaceConf = &InterfaceConf{
		PrivateKey: interfaceResponse.PrivateKey,
		Address:    interfaceResponse.Address,
		ListenPort: interfaceResponse.ListenPort,
		PostUp:     interfaceResponse.PostUp,
		PreDown:    interfaceResponse.PreDown,
	}

	var peerConfList []*PeerConf
	localIP := getLocalIP()
	for _, peer := range network.PeerList {
		if strings.HasPrefix(peer.Endpoint, localIP) {
			// this peer describes itself
			continue
		}
		peerConfList = append(peerConfList, &PeerConf{
			PublicKey:           peer.PublicKey,
			AllowedIPs:          peer.AllowedIPs,
			Endpoint:            peer.Endpoint,
			PersistentKeepalive: peer.PersistentKeepalive,
		})
	}
	// sort peerConfList by endpoint
	sort.Slice(peerConfList, func(i, j int) bool {
		return peerConfList[i].Endpoint < peerConfList[j].Endpoint
	})
	wgConf.peerConfList = peerConfList

	return wgConf
}

func syncWgConf(inputConf *WgConf) {
	var err error

	wgMutex.Lock()
	defer wgMutex.Unlock()

	err = prepareConfFile()
	checkErr(err)

	restartFlag := false
	if !checkServiceIsRunning() {
		restartFlag = true
	}

	interfaceConf, wholeConfStr, err := getCurrentConf()
	checkErr(err)
	if !compareInterfaceConf(interfaceConf, inputConf.interfaceConf) {
		restartFlag = true
	}

	if restartFlag {
		err = writeFile(confFilename, inputConf.generateString())
		checkErr(err)
		err = exec.Command("systemctl", "restart", serviceName).Run()
		checkErr(err)
	}

	inputConfStr := inputConf.generateString()
	if wholeConfStr != inputConfStr {
		// need sync
		err = writeFile(confFilename, inputConfStr)
		checkErr(err)
		err = exec.Command("wg syncconf wukuard <(wg-quick strip wukuard)").Run()
		checkErr(err)
	}
}

func buildPeerRequest() *pb.PeerRequest {
	return &pb.PeerRequest{
		Endpoint:   fmt.Sprintf("%s:9619", getLocalIP()),
		MacAddress: getMacAddress(),
		Hostname:   getHostname(),
	}
}

func clientMain(serverAddr, inputInterfaceName string) {
	parseServerAddr(serverAddr)
	log.Printf("INFO: try to connect to the server(%s:%s)......\n", serverIP, serverGrpcPort)
	interfaceName = inputInterfaceName
	conn, err := grpc.Dial(fmt.Sprintf("%s:%s", serverIP, serverGrpcPort), grpc.WithInsecure(), grpc.WithBlock())
	if err != nil {
		log.Fatalf("did not connect: %v", err)
	}
	defer conn.Close()
	c := pb.NewSyncNetClient(conn)

	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	for {
		<-t.C
		resp, err := c.HeartBeat(context.Background(), buildPeerRequest())
		if err != nil {
			log.Printf("ERROR: get error from grpc server: %s\n", err.Error())
		}
		syncWgConf(mapGrpcResponse(resp))
	}
}
