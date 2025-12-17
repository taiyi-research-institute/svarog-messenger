package messenger_test

import (
	"bytes"
	"encoding/gob"
	"encoding/hex"
	"reflect"
	"testing"
	"time"

	mr "github.com/taiyi-research-institute/svarog-messenger/messenger"
	"github.com/taiyi-research-institute/svarog-messenger/pb"
	// "google.golang.org/protobuf/proto"
)

func TestSessionConfig(t *testing.T) {
	hp := "127.0.0.1:65534"
	go func() {
		mr.SpawnServer(hp)
	}()
	time.Sleep(time.Second * 1)

	cl, err := new(mr.MessengerClient).Connect(hp)
	if err != nil {
		t.Fatalf("Cannot connect to %v, reason: %v", hp, err)
	}
	defer cl.Close()

	cfg, err := cl.GrpcNewSession(&pb.SessionConfig{
		Threshold: uint64(2),
		Players:   map[string]bool{"yaju": true, "sennpai": true, "tadokoro": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	// cfg_bytes, _ := proto.Marshal(cfg)
	cfg2, err := cl.GrpcGetSessionConfig(cfg.SessionId)
	if err != nil {
		t.Fatal(err)
	}
	// cfg2_bytes, _ := proto.Marshal(cfg2)
	if !reflect.DeepEqual(cfg, cfg2) {
		t.Fatal("cfg != cfg2")
	}
}

type Payload struct {
	X int
}

func TestGobToBytes(t *testing.T) {
	buf := new(bytes.Buffer)
	gob.NewEncoder(buf).Encode(&Payload{114514})
	t.Log(hex.EncodeToString(buf.Bytes()))
}

func TestGobFromBytes(t *testing.T) {
	dat, _ := hex.DecodeString("1a7f030101075061796c6f616401ff80000101010158010400000008ff8001fd037ea400")
	buf := new(bytes.Buffer)
	buf.Write(dat)

	obj := new(Payload)
	gob.NewDecoder(buf).Decode(obj)
	t.Log(obj)
}

func TestInboxOutbox(t *testing.T) {
	hp := "127.0.0.1:65534"
	go func() {
		mr.SpawnServer(hp)
	}()
	time.Sleep(time.Second * 1)

	cl, err := new(mr.MessengerClient).Connect(hp)
	if err != nil {
		t.Fatalf("Cannot connect to %v, reason: %v", hp, err)
	}
	defer cl.Close()

	// create dummy session
	cfg, err := cl.GrpcNewSession(&pb.SessionConfig{})
	if err != nil {
		t.Fatal(err)
	}
	sid := cfg.SessionId
	ddl := cfg.ExpireAtUnixEpoch
	t.Log(sid)

	// create slice of **not-nil** pointers to receive messages from others.
	vec_recv := make([]*Payload, 0)
	for i := range 5 {
		cl.RegisterSend(&Payload{X: i}, sid, "test1", 0, i, 0)
		recv := new(Payload)
		vec_recv = append(vec_recv, recv)
		cl.RegisterRecv(recv, sid, "test1", 0, i, 0)
	}
	cl.Exchange(ddl)

	for i, obj := range vec_recv {
		if obj.X != i {
			t.Fatal()
		}
	}

	map_recv := make(map[int]*[]byte)
	for i := range 5 {
		cl.RegisterSend([]byte{114, 51, 4}, sid, "test2", 0, i, 0)
		recv := new([]byte)
		map_recv[i] = recv
		cl.RegisterRecv(recv, sid, "test2", 0, i, 0)
	}
	cl.Exchange(ddl)

	for _, recv := range map_recv {
		r1 := hex.EncodeToString(*recv)
		r2 := hex.EncodeToString([]byte{114, 51, 4})
		if r1 != r2 {
			t.Log("r1", r1)
			t.Log("r2", r2)
			t.Fatal()
		}
	}

	cl.Close()
}
