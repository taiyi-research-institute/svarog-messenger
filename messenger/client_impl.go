package messenger

import (
	"bytes"
	"context"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"time"

	"golang.org/x/crypto/blake2b"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"github.com/taiyi-research-institute/svarog-messenger/pb"
)

const (
	BCAST_ID = 0
)

type MessengerClient struct {
	tx   []*pb.Message
	rx   map[string]any
	conn *grpc.ClientConn

	SessionId string
}

func (cl *MessengerClient) Connect(manager_hostport string) (*MessengerClient, error) {
	conn, err := grpc.NewClient(
		manager_hostport,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}
	if cl != nil {
		*cl = MessengerClient{
			tx:   make([]*pb.Message, 0),
			rx:   make(map[string]any),
			conn: conn,
		}
	}
	return cl, nil
}

func (cl *MessengerClient) Close() error {
	return cl.conn.Close()
}

func (cl *MessengerClient) stub() pb.MpcSessionManagerClient {
	return pb.NewMpcSessionManagerClient(cl.conn)
}

func (cl *MessengerClient) GrpcNewSession(
	cfg_req *pb.SessionConfig,
) (*pb.SessionConfig, error) {
	// ceremony
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	cfg_resp, err := stub.NewSession(ctx, cfg_req)
	if err != nil {
		return nil, err
	}
	cl.SessionId = cfg_resp.SessionId
	return cfg_resp, nil
}

func (cl *MessengerClient) GrpcGetSessionConfig(
	session_id string,
) (*pb.SessionConfig, error) {
	// ceremony
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	cfg, err := stub.GetSessionConfig(ctx, &pb.SessionId{Value: session_id})
	if err != nil {
		return nil, err
	}
	cl.SessionId = cfg.SessionId
	return cfg, nil
}

func (cl *MessengerClient) RegisterSend(obj any, sid string, topic string, src int, dst int, seq int) *MessengerClient {
	buf0 := new(bytes.Buffer)
	err := gob.NewEncoder(buf0).Encode(obj)
	if err != nil {
		panic(err)
	}
	key, buf := PrimaryKey(sid, topic, src, dst, seq), buf0.Bytes()
	cl.tx = append(cl.tx, &pb.Message{Key: key, Obj: buf})
	return cl
}

func (cl *MessengerClient) RegisterRecv(out any, sid string, topic string, src int, dst int, seq int) *MessengerClient {
	key := PrimaryKey(sid, topic, src, dst, seq)
	cl.rx[key] = out
	return cl
}

func (cl *MessengerClient) Exchange(ddl_unixepoch int64) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	_, err := stub.Inbox(ctx, &pb.VecMessage{Values: cl.tx})
	if err != nil {
		return err
	}

	ctx, cancel = context.WithDeadline(context.Background(), time.Unix(ddl_unixepoch, 0))
	defer cancel()

	req := &pb.VecMessage{Values: make([]*pb.Message, 0)}
	for k := range cl.rx {
		req.Values = append(req.Values, &pb.Message{Key: k, Obj: nil})
	}

	resp, err := stub.Outbox(ctx, req)
	if err != nil {
		return err
	}

	for _, msg := range resp.Values {
		ptr, ok := cl.rx[msg.Key]
		if !ok {
			err = fmt.Errorf("received a message with primary key %v which is not registered", msg.Key)
			return err
		}

		buf := bytes.NewBuffer(msg.Obj)
		err := gob.NewDecoder(buf).Decode(ptr)

		if err != nil {
			err = fmt.Errorf("MpcExchange cannot unmarshal msg.obj (2): %w", err)
			return err
		}
	}

	cl.MpcClear()
	return nil
}

func (cl *MessengerClient) TwoPartySend(obj any, sid string, seq int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	key := PrimaryKey(sid, "two party", 0, 0, seq)
	buf0 := new(bytes.Buffer)
	err := gob.NewEncoder(buf0).Encode(obj)
	if err != nil {
		err = fmt.Errorf("« TwoPartySend » failed to serialize object: sid = %s, seq = %s", sid, seq)
		return err
	}
	req0 := &pb.Message{Key: key, Obj: buf0.Bytes()}
	req := &pb.VecMessage{Values: []*pb.Message{req0}}

	if _, err = stub.Inbox(ctx, req); err != nil {
		err = fmt.Errorf("« TwoPartySend » failed to post object: sid = %s, seq = %d", sid, seq)
		return err
	}
	return nil
}

func (cl *MessengerClient) TwoPartyRecv(out any, sid string, seq int) error {
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	stub := cl.stub()

	key := PrimaryKey(sid, "two party", 0, 0, seq)
	req0 := &pb.Message{Key: key, Obj: nil}
	req := &pb.VecMessage{Values: []*pb.Message{req0}}

	resp0, err := stub.Outbox(ctx, req)
	if err != nil {
		err = fmt.Errorf("« TwoPartyRecv » failed to post object: sid = %s, seq = %d", sid, seq)
		return err
	}
	if len(resp0.Values) != 1 {
		err = fmt.Errorf("« TwoPartyRecv » received bad response: sid = %s, seq = %d", sid, seq)
		return err
	}
	resp := resp0.Values[0].Obj

	buf := bytes.NewBuffer(resp)
	err = gob.NewDecoder(buf).Decode(out)
	if err != nil {
		err = fmt.Errorf("« TwoPartyRecv » failed to deserialize object: sid = %s, seq = %d", sid, seq)
		return err
	}

	return nil
}

func (cl *MessengerClient) MpcClear() {
	cl.tx = make([]*pb.Message, 0)
	cl.rx = make(map[string]any)
}

func PrimaryKey(args ...any) string {
	ha, _ := blake2b.New256(nil)
	for _, arg := range args {
		buf := bytes.NewBuffer(nil)
		err := gob.NewEncoder(buf).Encode(arg)
		if err != nil {
			panic(err)
		}
		ha.Write(buf.Bytes())
	}
	return hex.EncodeToString(ha.Sum(nil))
}

// Thanks to
// https://github.com/grpc/grpc-go/blob/master/examples/route_guide/client/client.go
