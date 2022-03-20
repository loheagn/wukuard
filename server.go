package main

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"net"
	"os"
	"path/filepath"

	_ "github.com/go-sql-driver/mysql"
	pb "github.com/loheagn/wukuard/grpc"
	"google.golang.org/grpc"
	"gopkg.in/yaml.v3"
)

type ServerConfig struct {
	DB struct {
		Host     string `yaml:"host"`
		Name     string `yaml:"name"`
		User     string `yaml:"user"`
		Password string `yaml:"password"`
	} `yaml:"db"`
	Port string `yaml:"port"`
}

type PeerRecord struct {
	id                  int32
	macAddress          sql.NullString
	hostname            string
	token               sql.NullString
	publicKey           string
	privateKey          string
	postUP              string
	preDown             string
	address             string
	listenPort          int32
	endPoint            string
	allowedIPs          string
	persistentKeepalive int32
	createdAt           int64
	updatedAt           int64
}

var db *sql.DB

type server struct {
	pb.UnsafeSyncNetServer
}

func readPeerRecord(rows *sql.Rows) *PeerRecord {
	record := new(PeerRecord)
	err := rows.Scan(
		&(record.id),
		&(record.macAddress),
		&(record.hostname),
		&(record.token),
		&(record.publicKey),
		&(record.privateKey),
		&(record.postUP),
		&(record.preDown),
		&(record.address),
		&(record.listenPort),
		&(record.endPoint),
		&(record.allowedIPs),
		&(record.persistentKeepalive),
		&(record.createdAt),
		&(record.updatedAt),
	)
	if err != nil {
		log.Printf("ERROR: read peerRecord from DB: %s\n", err.Error())
		return nil
	}
	return record
}

func readPeerRecordList(rows *sql.Rows) []*PeerRecord {
	var recordList []*PeerRecord
	defer rows.Close()
	for rows.Next() {
		if record := readPeerRecord(rows); record != nil {
			recordList = append(recordList, record)
		}
	}
	return recordList
}

func fetchSelfRecord(macAddress, hostname string) (record *PeerRecord) {
	readFunc := func(col, value string) *PeerRecord {
		queryStr := fmt.Sprintf("select * from wukuard where %s = ?", col)
		rows, err := db.Query(queryStr, value)
		checkErr(err)
		defer rows.Close()
		recordList := readPeerRecordList(rows)
		if len(recordList) > 1 {
			log.Printf("ERROR: duplicate %s: %s\n", col, value)
			return nil
		}
		if len(recordList) <= 0 {
			return nil
		}
		return recordList[0]
	}
	colList := []string{"mac_address", "hostname", "token", "token"}
	valueList := []string{macAddress, hostname, macAddress, hostname}
	for i := range colList {
		if record = readFunc(colList[i], valueList[i]); record != nil {
			return
		}
	}
	log.Printf("WARN: failed to fetch peer record: %s, %s\n", macAddress, hostname)
	return
}

func fetchAllRecords() []*PeerRecord {
	rows, err := db.Query("select * from wukuard")
	checkErr(err)
	return readPeerRecordList(rows)
}

func updatePeerEndpoint(record *PeerRecord, endpoint string) *PeerRecord {
	_, err := db.Exec("update wukuard set endpoint=? where id=?", endpoint, record.id)
	if err != nil {
		checkErr(err)
		return nil
	}
	record.endPoint = endpoint
	return record
}

func (s *server) HeartBeat(_ context.Context, req *pb.PeerRequest) (*pb.NetWorkResponse, error) {
	resp := &pb.NetWorkResponse{}

	self := fetchSelfRecord(req.MacAddress, req.Hostname)
	if self == nil {
		return resp, nil
	}
	if self.endPoint != req.Endpoint {
		// update client peer info
		self = updatePeerEndpoint(self, req.Endpoint)
		if self == nil {
			return resp, nil
		}
	}
	resp.InterfaceResponse = &pb.InterfaceResponse{
		PrivateKey: self.privateKey,
		Address:    self.address,
		ListenPort: self.listenPort,
		PostUp:     self.postUP,
		PreDown:    self.preDown,
	}

	// build response
	records := fetchAllRecords()
	peerList := make([]*pb.PeerResponse, 0)
	for _, v := range records {
		if v.id == self.id {
			continue
		}
		peerList = append(peerList, &pb.PeerResponse{
			Endpoint:            v.endPoint,
			PublicKey:           v.publicKey,
			AllowedIPs:          v.allowedIPs,
			PersistentKeepalive: v.persistentKeepalive,
		})
	}

	resp.PeerList = peerList
	return resp, nil
}

func serverMain(confPath string) {
	var err error
	confPath, err = filepath.Abs(confPath)
	if err != nil {
		panic("invalid config path")
	}
	if _, err = os.Stat(confPath); err != nil {
		panic(fmt.Sprintf("invalid config path: %s", err.Error()))
	}
	confFileBytes, err := os.ReadFile(confPath)
	if err != nil {
		panic(err)
	}
	conf := &ServerConfig{}
	err = yaml.Unmarshal(confFileBytes, conf)
	if err != nil {
		panic(err)
	}

	dbConf := conf.DB
	db, err = sql.Open("mysql", fmt.Sprintf("%s:%s@tcp(%s)/%s", dbConf.User, dbConf.Password, dbConf.Host, dbConf.Name))
	if err != nil || db.Ping() != nil {
		panic(err)
	}

	if conf.Port == "" {
		panic("invalid port")
	}

	lis, err := net.Listen("tcp", "0.0.0.0:"+conf.Port)
	if err != nil {
		log.Fatalf("failed to listen: %v", err)
	}
	s := grpc.NewServer()
	pb.RegisterSyncNetServer(s, &server{})
	log.Printf("server listening at %v", lis.Addr())
	if err := s.Serve(lis); err != nil {
		log.Fatalf("failed to serve: %v", err)
	}
}
