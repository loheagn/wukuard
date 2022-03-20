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

func getCurrentConf() (string, error) {
	wholeConfStr, err := readFile(confFilename)
	if err != nil {
		return "", err
	}
	return wholeConfStr, nil
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
	conn, err := net.Dial("udp", serverIP+":80")

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

	if inputConf == nil || inputConf.interfaceConf == nil {
		err = writeFile(confFilename, "")
		checkErr(err)
		if checkServiceIsRunning() {
			log.Println("INFO: stop wukuard service")
			_ = exec.Command("/bin/bash", "-c", "ip link delete wukuard").Run()
			err = exec.Command("systemctl", "stop", serviceName).Run()
			checkErr(err)
		}
		return
	}

	err = prepareConfFile()
	checkErr(err)

	wholeConfStr, err := getCurrentConf()
	inputConfStr := inputConf.generateString()
	if wholeConfStr != inputConfStr || !checkServiceIsRunning() {
		err = writeFile(confFilename, inputConfStr)
		checkErr(err)
		log.Println("INFO: restart wukuard service")
		err = exec.Command("systemctl", "restart", serviceName).Run()
		if err != nil {
			// stupid but effective
			_ = exec.Command("/bin/bash", "-c", "ip link delete wukuard").Run()
			err = exec.Command("systemctl", "restart", serviceName).Run()
			checkErr(err)
		}
		return
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
		log.Fatalf("ERROR: did not connect: %v", err)
	}
	log.Printf("INFO: connected to the server(%s:%s)......\n", serverIP, serverGrpcPort)
	defer conn.Close()
	c := pb.NewSyncNetClient(conn)

	t := time.NewTicker(10 * time.Second)
	defer t.Stop()
	defer func() {
		_ = exec.Command("/bin/bash", "-c", "ip link delete wukuard").Run()
		err = exec.Command("systemctl", "stop", serviceName).Run()
		checkErr(err)
	}()
	for {
		<-t.C
		resp, err := c.HeartBeat(context.Background(), buildPeerRequest())
		if err != nil {
			log.Printf("ERROR: get error from grpc server: %s\n", err.Error())
		}
		syncWgConf(mapGrpcResponse(resp))
	}
}
