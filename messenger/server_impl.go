package messenger

import (
	"log"
	"net"

	"github.com/taiyi-research-institute/svarog-messenger/pb"

	"context"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/patrickmn/go-cache"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

const SESSION_TIMEOUT = time.Second * 60

func SpawnServer(host string, port uint16) {
	hp := fmt.Sprintf("%s:%d", host, port)
	sock, err := net.Listen("tcp", hp)
	if err != nil {
		log.Println("failed to listen to", hp)
		panic(err)
	}

	grpc_server := grpc.NewServer(
		grpc.MaxRecvMsgSize(1048576 * 32),
	)
	// See `func main()` of the following link to learn how to configure HTTPS.
	// https://github.com/grpc/grpc-go/blob/master/examples/route_guide/server/server.go

	pb.RegisterMpcSessionManagerServer(grpc_server, NewServer())
	grpc_server.Serve(sock)
}

type MessengerServer struct {
	pb.UnimplementedMpcSessionManagerServer
	void *pb.Void
	db   *cache.Cache
	db2  *cache.Cache
}

func NewServer() *MessengerServer {
	s := &MessengerServer{}
	s.void = &pb.Void{}
	s.db = cache.New(SESSION_TIMEOUT, 180*time.Second)
	return s
}

func (s *MessengerServer) NewSession(
	ctx context.Context,
	cfg *pb.SessionConfig,
) (*pb.SessionConfig, error) {
	// If field `SessionId` is not provided,
	// then create with UUID-v7, in lowercase hex string --WITHOUT-- hyphens.
	if cfg.SessionId == "" {
		uuidv7, _ := uuid.NewV7()
		sid_buf, _ := uuidv7.MarshalBinary()
		sid_str := hex.EncodeToString(sid_buf) // lowercase hex without hyphen.
		cfg.SessionId = sid_str
	}

	// Store expiration time for further use.
	cfg.ExpireAtUnixEpoch = time.Now().Add(SESSION_TIMEOUT).Unix()

	s.db.Add(cfg.SessionId, cfg, cache.DefaultExpiration)

	return cfg, nil
}

func (s *MessengerServer) GetSessionConfig(
	ctx context.Context,
	req *pb.SessionId,
) (*pb.SessionConfig, error) {
	obj, session_found := s.db.Get(req.Value)
	if !session_found {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Session %s does not exist", req.Value))
	}
	cfg, _ := obj.(*pb.SessionConfig)
	return cfg, nil
}

func (s *MessengerServer) Inbox(
	ctx context.Context,
	req *pb.VecMessage,
) (*pb.Void, error) {
	vec_msg := req.Values
	for _, msg := range vec_msg {
		_, found := s.db.Get(msg.Key)
		if !found {
			s.db.Add(msg.Key, msg.Obj, cache.DefaultExpiration)
		} else {
			err := status.Error(codes.AlreadyExists, fmt.Sprintf("message with key %s already exists", msg.Key))
			return nil, err
		}
	}
	return s.void, nil
}

func (s *MessengerServer) Outbox(
	ctx context.Context,
	req *pb.VecMessage,
) (*pb.VecMessage, error) {
	vec_req := req.Values
	vec_resp := &pb.VecMessage{Values: make([]*pb.Message, len(vec_req))}
	for i, req := range vec_req {
		obj, found := s.db.Get(req.Key)
		for !found {
			// #region prevent from running forever
			ddl, ok := ctx.Deadline()
			if ok {
				if time.Now().Compare(ddl) > 0 {
					err := status.Error(codes.DeadlineExceeded, req.Key)
					return nil, err
				}
			}
			// #endregion

			time.Sleep(250 * time.Millisecond)
			obj, found = s.db.Get(req.Key)
		}
		vec_resp.Values[i] = &pb.Message{
			Key: req.Key,
			Obj: obj.([]byte),
		}
	}
	return vec_resp, nil
}

func (s *MessengerServer) Ping(
	ctx context.Context,
	req *pb.Void,
) (*pb.PingResponse, error) {
	msg := &pb.PingResponse{
		Value: "Svarog Session Manager is running.",
	}
	return msg, nil
}
